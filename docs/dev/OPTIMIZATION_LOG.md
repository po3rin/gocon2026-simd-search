# SIMD 最適化の記録 — 「速くならない」を一つずつ潰す

ワークショップ教材用メモ。Go 1.26 `simd/archsimd` でベクトル検索を高速化する過程で
実際に踏んだ罠と、その診断・修正の記録。すべて実測ベース。

> **読み方**: このファイルは「罠と診断の生ログ」。各 Step が**ルーフライン上で
> どの天井に当たっていたか**は [参加者教材 workshop.md](../workshop/workshop.md) に整理してある。
> 対応の目安: Step 0/1 = AI 0.5 でメモリ斜線に張り付く(縦に上る)、Step 3 = byte を
> 削って横に動く、Step 4(VZEROUPPER) = メモリ天井に届く前の隠れ sub-ceiling の掃除。

測定環境: AWS c7i.large(Xeon Platinum 8488C / Sapphire Rapids、AVX-512 + VPOPCNTDQ あり)、
Go 1.26.4、`GOEXPERIMENT=simd`。ベンチは 384 次元 float32、検索は 10 万ベクトル。

---

## Step 0: ベースライン

スカラー実装(`DotNaive`、`Hamming`)と全探索 `Index` を実装。

| ベンチ | 結果 |
|---|---|
| DotNaive(カーネル) | 205 ns |
| SearchNaive(10万件全探索) | 27.3 ms(5.6 GB/s) |

**教材ポイント**: naive のスカラーループは `sum += a[i]*b[i]` の加算レイテンシ
(Sapphire Rapids で 2 cycle)の直列チェーンで律速。384 要素 × 2 cycle ≒ 205ns と計算が合う。

## Step 1: 素朴な SIMD 化 → まさかの 1.0x(罠 ①②)

`LoadFloat32x8Slice(a[i:])` + `MulAdd`(FMA)、アキュムレータ1本の素直な実装。

| ベンチ | 結果 | 期待 |
|---|---|---|
| DotSIMD(カーネル) | 219 ns(**naive より遅い!**) | 〜8x |
| SearchSIMD(全探索) | 28.5 ms(**1.0x**) | 〜8x |

ここに**2つの独立した問題**が隠れていた:

### 罠①: 全探索はメモリ帯域律速(SIMD 以前の問題)

10万 × 384次元 × 4B = **153MB はキャッシュに乗らない**。naive ですら DRAM 帯域
(この VM で実測 〜5.6GB/s)に張り付いており、ALU をいくら速くしても変わらない。

> **SIMD は「データが届いている」ときにしか効かない。**
> → だからデータを小さくする量子化(Step 3)が本命になる。

### 罠②: カーネル単体でも遅い → objdump で犯人捜し

`GOARCH=amd64 GOEXPERIMENT=simd go test -c` + `go tool objdump` でループ本体を見ると、
ベクトル命令 3 つ(VMOVDQU×2 + VFMADD213PS)に対して、`a[i:]` のスライス式から生成された
**境界計算(SUBQ/SARQ/ANDQ/LEAQ…)が約20命令**ぶら下がっていた。
さらにアキュムレータ1本だと FMA のレイテンシ(4 cycle)が直列化する。

※ 小ネタ: `go tool objdump` は VEX エンコードの一部を正しくデコードできず、
`VFMADD213PS` が `TESTL $0xd1, AL` のような文字化けで表示される。命令バイト列
(`c4e265a8d1`)を手で読むと正体がわかる。

## Step 2: スライス前進 + アキュムレータ2本(部分的成功)

- `a[i:]` でインデックスせず `a = a[16:]` と前進(境界計算をループ条件に吸収させる狙い)
- アキュムレータ 2 本で FMA チェーンを分割

| ベンチ | Before | After | 倍率(vs scalar) |
|---|---|---|---|
| DotSIMD | 219 ns | 188 ns | **1.1x — まだダメ** |
| HammingSIMDBig(4096語) | 1545 ns | 949 ns | **2.6x ✅**(scalar 2435ns) |
| HammingSIMD(6語) | 6.1 ns | 5.8 ns | 1.0x(オーバーヘッド支配) |

**教材ポイント**:
- popcount 側はアキュムレータ分割が素直に効いた
- Dot は IPC ≒ 1.0 のまま。スライス前進でも、コンパイラが nil/長さ防御のための
  ポインタ補正(NEGQ/SARQ/ANDQ)を毎周生成し、ロードアドレスの依存チェーンになっている疑い
- **ハミング距離は 6 語(384bit)では SIMD の意味がない**。1呼び出しの仕事が小さすぎて
  水平和などの固定費が支配する。SIMD popcount はブロックをまとめて処理するときに効く

## Step 3: バイナリ量子化(本命の 40x)

float32 → 符号1bit。153MB → **4.8MB(1/32)でキャッシュに乗る**。

| ベンチ | 結果 | vs naive |
|---|---|---|
| SearchBinary(スカラー popcount) | 0.65 ms | **40.8x** 🚀 |
| SearchBinaryRerank(k×10 を float32 再採点) | 0.73 ms | **36.1x** |
| Recall@10(クラスタ合成データ) | binary 単体 0.18 → rerank 後 **0.87** | |

**教材ポイント**:
- 高速化の主因は SIMD ではなく**データ表現の変更**(メモリ 1/32 + XOR/POPCNT の安さ)
- `math/bits.OnesCount64` はスカラーでも POPCNT 命令 1 発。スカラーで十分速い
- 精度の犠牲(Recall 0.18)は安い再ランクで 0.87 まで回復する。
  「速度と精度は二者択一ではない」(ClickHouse QBit と同じ思想)

### 罠③(おまけ): 合成データの作り方で recall が壊れる

クラスタ中心を先に正規化すると成分(〜1/√384 ≒ 0.05)がノイズ(0.4)に埋もれ、
符号ビットがほぼ乱数になって binary 検索が成立しない(recall 0.36)。
中心は N(0,1) のまま使い、最後にベクトル全体を正規化するのが正しい。

## Step 4: Dot コード生成ラボ → 境界チェック仮説の棄却

罠②の残りを特定するため、同じ計算を4通りの書き方で比較(`dot_lab_test.go`):

| variant | 書き方 | 検証したい仮説 |
|---|---|---|
| idx2 | `a[i:]` インデックス + 2acc | ベースライン(Step 1 相当 + 2acc) |
| arr2 | `(*[8]float32)(a[i:i+8])` 配列ポインタ変換 | 境界チェックが消えるか |
| unsafe2 | `unsafe.Add` ポインタ演算 + 2acc | 境界チェックゼロの理論値 |
| unsafe4 | 同上 + 4acc | FMA レイテンシ隠蔽の上限 |

結果(384次元、naive = 204ns):

| variant | ns/op | 倍率 |
|---|---|---|
| idx2 | 220 | 0.93x |
| arr2 | 180 | 1.13x |
| unsafe2 | 169 | 1.21x |
| unsafe4 | 166 | **1.23x** |

**仮説棄却**: 境界チェックを完全に消して(unsafe)アキュムレータを4本にしても 1.23x。
スライス境界計算は主犯ではなかった(寄与は 220→166ns の差分ぶんだけ)。
全 variant に共通する**約140nsの固定費**が残っている。

### Step 4b: 固定費の切り分け → 「呼び出しごとに約145ns」が確定

dim を 4096 に上げて固定費を 1/10 に薄めたところ、SIMD が本気を出した:

| dim | naive | Dot(現行) | unsafe4 | unsafe4 倍率 |
|---|---|---|---|---|
| 384 | 208 ns | 186 ns | 163 ns | 1.2x |
| 4096 | 2386 ns | 589 ns | **354 ns** | **6.7x** 🎉 |

2点の連立方程式から逆算すると、**1呼び出しあたりの固定費 ≒ 145ns(550サイクル)**。
ループ本体は 384 要素で 20ns 程度しかかかっていない。
SIMD ループ自体はずっと正しく速かった — 呼び出しの「入口か出口」で何かが燃えている。

**教材ポイント**: ベンチのスケールを変えて測ると「比例する部分」と「固定の部分」を
分離できる。これは prof より先にやるべき基本技。

### Step 4c: 固定費の犯人捜し

容疑者を順に取り調べ:

1. **`buf` のヒープエスケープ?** → `-gcflags=-m` でシロ(エスケープなし)
2. **水平和のスカラー演算そのもの?** → 高々 30 サイクル程度のはずでシロ寄り
3. **VEX(ymm)↔レガシーSSE 混在ペナルティ?** → 現在の本命。
   `go tool objdump` で確認すると、生成コードに **VZEROUPPER が 1 個もない**。
   Go はデフォルト(`GOAMD64=v1`)でスカラー浮動小数点をレガシー SSE
   (`MOVSS`/`ADDSS`)で出すため、ymm の上位ビットを汚した直後にレガシー SSE を
   実行する形になっている。このペナルティの仕組みは世代で異なる:
   Sandy/Ivy Bridge〜Haswell では「一度きりの数百サイクルのモード切替
   (マイクロコードアシスト)」だったが、**Skylake 以降の modern Intel では
   仕組みが変わり、dirty 状態で実行するレガシー SSE 命令1個ごとにペナルティ
   (partial-register 依存 + blend の挿入)が付く形**になっている。本ループの
   水平和は毎呼び出しこの per-instruction ペナルティを踏むため、145ns はつじつまが合う

検証実験1: **`GOAMD64=v3` でビルド** → 結果は**全ベンチ誤差レベルで不変**。

…が、これは**仮説の棄却ではなく実験の無効**だった。objdump で確認すると
**`GOAMD64=v3` でもスカラー浮動小数点はレガシー SSE(`f30f59` MULSS)のまま**。
Go コンパイラ(1.26)はスカラー float に VEX エンコードを使わない。

**教材ポイント**: 「実験で差が出ない」には2通りある — 仮説が間違っているか、
実験が仮説に触れていないか。介入が効いたことを必ず objdump 等で確認してから結論を出す。

### Step 4d: VZEROUPPER 直接実験 → 真犯人確定

ここで仮説の整合性が一気に揃う:

- `BenchmarkHammingSIMD`(カーネル)5.8ns — インライン化され、VEX 命令の間に
  レガシー SSE float が挟まらない → ペナルティなし
- `SearchBinarySIMD` 内の同じ関数は 137ns/call — ループ内の topK が
  **float32 比較(レガシー SSE)を毎回実行** → 毎呼び出し遷移ペナルティ
- Dot 系は関数内部の水平和が必ずスカラー SSE → 常に〜145ns の税金
- **「未解決の謎」だった SearchBinarySIMD の異常もこの仮説1本で説明できる**

検証実験2: アセンブリ1行の `vzeroupper()` 関数を作り(Go は挿入してくれないので自作)、
ベクトル→スカラーの境界に置いた `dotUnsafeVZ2` を追加。

**結果: 確定。**

| variant | ns/op | vs naive (206ns) |
|---|---|---|
| unsafe2(VZなし) | 167.4 | 1.2x |
| **unsafeVZ2(VZEROUPPER 1命令追加)** | **23.4** | **8.8x** 🎉🎉🎉 |

VZEROUPPER 1命令(自体のコストは1〜2cycle)で同一コードが **7.1倍**速くなった。
145ns の固定費の正体は「dirty ymm 上位 × レガシー SSE」の遷移ペナルティで確定。

### 結論(この調査の最重要知見)

> **Go 1.26 の `GOEXPERIMENT=simd` は VZEROUPPER を自動挿入しない。**
> SIMD 関数とスカラー float コード(自関数内の水平和、呼び出し元の比較処理など)が
> 混ざる境界では、手書きアセンブリの `VZEROUPPER` を呼ばないと、
> 新しめの Intel では SIMD 関数呼び出しごとに〜550 cycle の隠れ税を払う。
> 症状は「SIMD化したのに naive と同速」であり、原因箇所はプロファイラに直接出ない。

対応:
- `vec.Dot` / `vec.HammingSIMD` の水平和直前に `vzeroupper()` を挿入(本実装に反映済み)
- `vzeroupper_amd64.s` はわずか3行のアセンブリ
- ※ AMD(Zen系)にはこのペナルティは無い/軽微とされる。Intel 固有の罠
- ※ コンパイラが自動挿入すべきもの。golang/go への issue 報告候補

**教材ポイント(ワークショップの山場)**:
1. SIMD カーネルを正しく書いても、**ABI/呼び出し境界の1命令**で台無しになる
2. 仮説→実験のループ: 境界チェック説(棄却)→ GOAMD64=v3(実験無効)→
   dim スケーリングで固定費を分離 → VZEROUPPER で確定。プロファイラに出ない問題は
   「介入実験」でしか特定できない

## Step 5: VZEROUPPER 反映後の最終計測

`vec.Dot` と `vec.HammingSIMD` に `vzeroupper()` を入れて全体を取り直した結果:

### カーネル

| ベンチ | Before | After | vs naive |
|---|---|---|---|
| DotSIMD(384次元) | 186 ns | **44.7 ns** | **4.6x** 🎉 |
| (参考)unsafe4 + VZ | — | 23.5 ns | 8.7x(bonus 章ネタ) |
| HammingSIMDBig | 949 ns | 918 ns | 2.6x |
| HammingSIMD(6語) | 5.8 ns | 7.0 ns | 0.66x(小ブロックでは SIMD 不利のまま) |

### 検索(10万件 × 384次元)

| エンジン | Before | After | vs naive |
|---|---|---|---|
| SearchNaive | 28.6 ms(5.4 GB/s) | 同左 | 1.0x |
| SearchSIMD | 26.3 ms | **18.0 ms(8.6 GB/s)** | **1.6x** |
| SearchBinary | 0.65 ms | 0.68 ms | **42.3x** 🚀 |
| SearchBinarySIMD | **13.7 ms(異常)** | **0.75 ms(正常化!)** | 38.3x |
| SearchBinaryRerank | 0.73 ms | 0.73 ms | **39.2x**(Recall@10 0.87) |

**教材ポイント**:
- 「未解決の謎」だった SearchBinarySIMD の異常(137ns/call)は VZEROUPPER で完全解決。
  topK の float 比較(レガシー SSE)に毎回突っ込んでいたのが原因で、仮説どおりだった
- もうひとつの発見: Step 1 で「メモリ帯域の壁 5.6GB/s」と書いたが、**あれは遷移ペナルティ
  込みの偽の壁**だった。真の DRAM 限界はこの VM で〜8.6GB/s で、SearchSIMD はそこに到達
  (1.0x → 1.6x に改善)。**ボトルネック分析は一枚岩ではなく、剥がすと次の層が出てくる**
- それでもカーネル 4.6x → 全探索 1.6x の落差は健在。メモリの壁の教材はそのまま成立し、
  量子化(データ 1/32)の 42x が本命であることも変わらない

## Step 6: ルーフラインの天井を実測する → Go archsimd の register spill 発見(2026-06-13)

「天井は推定でなく実測すべき」という指摘を受け、ルーフラインの上限そのものを c7i で
マイクロベンチした(`make remote-roofline-ceiling` / `make remote-roofline`、同一セッション)。
測り方は `../workshop/workshop.md` §2.5、流儀は LBNL ERT / Williams 2009(同 §10)に準拠。

### 生ログ(c7i.large / Xeon 8488C / 負荷時 〜3.75GHz / Go 1.26.4)

```
# 天井(マイクロベンチ)
BenchmarkPeakFLOP_AVX2   39.31 GFLOP/s      ← FMA 飽和(レジスタ上だけ、acc 12本)
BenchmarkPeakReadBW       5.92 read-GB/s    ← 順次read(スカラ縮約)
BenchmarkPeakTriadBW     10.98 triad-GB/s   ← STREAM Triad

# 検索3点(同一セッション)
SearchNaive   26.97 ms   5.70 GB/s   2.85 GFLOP/s   AI 0.5
SearchSIMD    16.65 ms   9.22 GB/s   4.61 GFLOP/s   AI 0.5   ← Triad の84%、ほぼ壁
SearchBinary   0.62 ms   (4.8 MB/query)             43.3x
```

### 罠④: 演算ピークが AVX2 理論(〜120GF)の約1/3しか出ない

⚠️ 当初「シリコン理論240の16%」と書いたが、これは**比較対象の取り違え**だった。
本ベンチは **AVX2(256bit, 8レーン)** なので理論ピークは
`2 FMA/cyc × 8 lane × 2 flop × 3.75GHz ≒ 120 GFLOP/s`(240 は AVX-512=16レーンの値)。
だが実測は **39 GF = AVX2ピークの約1/3(33%)**。`39.3/(8×2)=2.46 GFMA/s ÷ 3.75GHz ≒ 0.66 FMA/cycle`
— ハードは 2 FMA/cycle 出せるのに 0.66 しか出ていない。

`go tool objdump` で内側ループ(独立アキュムレータ12本)を見ると、各 FMA がこうなっていた:

```
VMOVDQU 0x398(SP), X2     # acc をスタックからロード
c4e27da8d1 (VFMADD213PS)  # FMA(go objdump は VEX を誤デコードし TESTL 表示)
VMOVDQU X2, 0x398(SP)     # acc をスタックへ書き戻し
```

これが 12 本**全部**。しかも**全 acc が同一レジスタ X2 を使い回す**。
独立鎖のはずが load→FMA→store の往復(store-to-load forwarding 〜5–7cyc)で**直列化**し、
**FMA ユニットではなくスタック往復のレイテンシで律速**していた。

**結論**: Go 1.26 の `archsimd` は SIMD 値をレジスタに保持できず毎回スタックへ退避する
(**register spill** = レジスタに収まらない/置けない値をメモリ=スタックへ追い出すこと)。
イントリンシック自体は正しい命令(VFMADD213PS)に落ちるが、その周りのレジスタ割り当てが
未熟。**VZEROUPPER 未挿入(Step 4)と同根のコード生成の問題**。

**教材ポイント**:
- 「天井」は推定でなく実測する。しかも**そのベンチがハードを飽和できているか**を objdump で疑う
  (read ベンチ 5.9 が SIMD 全探索の達成 9.2 を下回るのも同種の「ベンチが過小」)
- ただし**この発見は結論を変えない**。内積カーネルは AI=0.5 で深いメモリ律速なので、
  演算天井が 39 でも 120 でも 240 でもリッジの遥か左 = メモリ律速のまま。ルーフラインの強さは
  「天井の絶対値が多少ブレても、点が壁に張り付いている事実は揺るがない」こと
- 「Go 1.26 archsimd は SIMD 値を register spill(スタックへ退避)する」現象は **既存 issue #76969** と同件
  (詳細と将来見通しは下の Step 6b)

### Step 6b: 「どう治すか」の切り分け — レジスタ圧 vs 根本

スピルが「アキュムレータが多すぎてレジスタに収まらない(=減らせば直る)」のか
「archsimd 値が常にメモリ常駐(=本数と無関係)」のかを、4本版を足して比較した。

| variant | acc | ns/op | GFLOP/s |
|---|---|---|---|
| `BenchmarkPeakFLOP_AVX2` | 12 | 20073 | **39.2** |
| `BenchmarkPeakFLOP_AVX2_4acc` | 4 | 11435 | 22.9 |

- **本数を減らすと逆に遅くなった**(39→23)。12本(+m+c=14)は ymm 16本に収まるのに
  スピルしており、4本版も同じ load→FMA→store パターン。→ **レジスタ圧の問題ではない**。
  archsimd 値が常にメモリ常駐で、本数を増やすほど OOO が独立な store-forward 鎖を
  並列に走らせて稼ぐ(が、各 FMA のスタック往復が頭打ちを作る)。

**どう治すか**:
1. **本筋はコンパイラ側(upstream)で、既に認識されている**。`GOEXPERIMENT=simd` は実験機能で、
   SIMD 値をベクトルレジスタに保持し続けるレジスタ割り当てが未成熟。**既存 issue**:
   - [#76969](https://github.com/golang/go/issues/76969) "Generated code doesn't use Y15 but
     spills register to stack" — 本件とまったく同じ症状(closed: NOT_PLANNED = 既知の制限扱い)
   - [#78753](https://github.com/golang/go/issues/78753) "AMD64 AVX-512 register allocation is
     limited to lower 16 ZMM registers"(**milestone Go1.27**, fixed)
   - [#78138](https://github.com/golang/go/issues/78138) archsimd codegen 改善(**Go1.27**)
   - [#77647](https://github.com/golang/go/issues/77647) Go/asm 境界オーバーヘッド(VZEROUPPER 系の話題)

   → **新規報告は不要(#76969 が同件)**。レジスタ割り当ては Go1.27 で改善が入りつつあり、
   **将来のバージョンでこの 39GF は上がる可能性が高い**。本リポの数値も Go 更新時に再計測する。
2. **今できる緩和**(完治はしない):
   - アキュムレータを**増やす**(減らすと悪化。12 > 4)で OOO に並列性を与える
   - カーネルを**長く**して呼び出し固定費を薄める(Step 4b の dim=4096 と同じ)
   - 本当に演算律速な処理は当面**手書きアセンブリ(Avo/.s)**が archsimd より速い
3. **このワークショップでは無視してよい**。内積カーネルは AI=0.5 の深いメモリ律速で、
   演算天井が 39 でも 120 でも 240 でも結論(量子化が本命)は変わらない。「コンパイラの未熟さすら
   ルーフラインの上では些末」というのが綺麗なオチ。

## Step 7: Codespaces(AMD EPYC 7763 / Zen3)で本編を再計測(2026-06-14)

本編(Stage 0/1/2 + rerank)を Codespaces 一本化したので、実際に **8-core Codespace
(`premiumLinux`)** を立てて本編フルを取り直した。当たった CPU は **AMD EPYC 7763
(Zen3、AVX2+FMA あり / AVX-512 VPOPCNTDQ なし)**。手順は [CODESPACES.md](CODESPACES.md)。
これで「本編は Codespaces のどの CPU でも再現／AVX-512 は保証されない」を実機で確認できた
(付録 `SearchBinarySIMD` は VPOPCNT 非搭載のため fallback)。

### 生ログ(8-core EPYC 7763 / Go 1.26.4 / `GOEXPERIMENT=simd` / 10万×384次元)

```
# 検索(本編フル, make bench)
SearchNaive         35.59 ms   4.32 GB/s   2.16 GFLOP/s   AI 0.5
SearchSIMD           8.41 ms  18.26 GB/s   9.13 GFLOP/s   AI 0.5   ← 4.2x(!)
SearchBinary         0.762 ms (4.8 MB/query)                       46.7x
SearchBinaryRerank   0.822 ms                                      43.3x
Recall@10:  binary 0.180  →  binary+rerank 0.868

# 天井(make roofline-ceiling)
PeakFLOP_AVX2(12acc)  25.57 GF     PeakFLOP_AVX2(4acc)  13.41 GF  ← 12>4 は c7i と同傾向(spill)
PeakReadBW             6.13 GB/s   PeakTriadBW           4.18 GB/s  ← どちらも過小(下記)

# バッチ(make roofline-batch)
SearchSIMD(B=1)        8.18 ms/q   9.39 GF   AI 0.5
SearchBatchNaive(B=32) 34.12 ms/q  2.25 GF   AI 16
SearchBatchSIMD(B=32)   5.89 ms/q 13.05 GF   AI 16   ← batch内 naive比 5.8x
```

### c7i との違い ―― 「SIMD 全探索」の倍率は CPU で変わる(数値を約束しない設計の裏付け)

| | c7i (Xeon 8488C / Sapphire Rapids) | Codespaces (EPYC 7763 / Zen3) |
|---|---|---|
| SIMD 全探索 | 16.7 ms / **1.6x**(メモリ斜線に張り付き) | 8.41 ms / **4.2x** |
| naive 全探索 | 27.0 ms (5.7 GB/s) | 35.6 ms (4.3 GB/s) |
| バイナリ量子化 | 0.62 ms / 43x | 0.762 ms / **46.7x** |

- **核は不変**: SIMD 全探索は最後はメモリ帯域に頭打ち(EPYC でも 18.3 GB/s で plateau)、
  量子化(46.7x)が本命、という二本柱は両 CPU で成立。
- **倍率だけが動く**: c7i は naive も SIMD もメモリ天井近く(5.7→9.2 GB/s)で差 1.6x。
  EPYC は **naive がレイテンシ律速で帯域を使い切れず(4.3 GB/s)**、SIMD が 18.3 GB/s まで
  伸ばすので差 4.2x。同じ「SIMD はメモリ天井で止まる」でも、naive の出発点が違うと倍率が変わる。
  → **「点を各自のベンチで打って確かめる(数値を約束しない)」というワークショップ設計の良い実例**。
- **天井ベンチの過小 → SIMD 化で解消(対応済み)**: 上の生ログのスカラ版 PeakReadBW 6.13 /
  PeakTriad 4.18 GB/s は、SIMD 全探索の実達成 18.3 GB/s を下回っていた(Step 6 と同じ「縮約ベンチが
  ハードを飽和できていない」問題で EPYC ではより顕著)。**天井ベンチを検索カーネルと同じ 256bit
  SIMD ロードに作り直して測り直した**(`ceiling_mem_{simd,scalar}_test.go`):
  - **PeakReadBW 6.13 → 18.39 GB/s、PeakTriad 4.18 → 16.26 GB/s**。
  - 検索は読むだけ(書き戻さない)なので壁は **read 帯域 18.4**。SearchSIMD 18.3 GB/s = read 天井の
    ~99%、達成 9.2 GFLOP/s = 0.5×18.4 と一致 → **ルーフライン上で点が壁に張り付く**(c7i より綺麗)。
  - これで「点が屋根の上に来てルーフラインが破綻」する問題も解消。リッジ = 25.5/18.4 ≈ 1.4。

### Step 7b: workshop を Codespaces ネイティブに改稿(対応済み・2026-06-15)

上の「c7i 前提の数字・演出」と Codespaces(EPYC) のズレを、教材側を直して解消した(commit `b4576af`):
- workshop.md の数値・ルーフライン図を EPYC 基準に統一(read 帯域=壁、リッジ 1.4、Stage1=4.2x で壁到達、
  Stage2 バッチ内 SIMD 5.8x、量子化 47x、rerank 43x)。図 6 枚(rl-stage0〜4 / roofline-plot)も再描画。
- c7i 前提の付録(VZEROUPPER 税 / register spill)は本編から外し [`HIDDEN_CEILINGS.md`](HIDDEN_CEILINGS.md) へ分離。
- Stage 1 の演出は「1.6x 止まり=無力」→「効いた(4.2x)が壁に張り付く・倍率は CPU 次第」に再フレーム。
  倍率は機械依存だが「最後はメモリ壁で頭打ち/量子化が本命」という骨格は不変。

## Step 8: レビュー反映 — 自作 asm を標準 API へ・教材とコマンドの整合(2026-07-07)

計測ではなく、ワークショップ資料レビューを受けたコード/教材の洗練。数値の変更はない。

- **`vzeroupper_amd64.s`(自作3行アセンブリ)を削除**し、全呼び出しを Go 1.26 標準の
  `archsimd.ClearAVXUpperBits()`(中身は同じ VZEROUPPER 1命令)に置換。
  「アセンブリも cgo も書かない」という教材の主張が完全になった。
  amd64 クロスコンパイルの `-S` 出力で VZEROUPPER が同数(7箇所)生成されることを確認済み。
- **register spill の説明を正確化**: 「16本の Y レジスタ」は誤りで、Y15 は Go 内部 ABI の
  予約ゼロレジスタのため**使えるのは15本**([golang/go#76969](https://github.com/golang/go/issues/76969)
  で not planned としてクローズ)。12本+m+c=14 は 15 本に収まる数、という論旨は変わらず。
- **`make bench1` にカーネル単体ベンチ(`BenchmarkDot(Naive|SIMD)`)を追加**。
  資料の 339ns → 55.9ns(6.1x)を参加者が再現できるコマンドが無かったため。
- **`make roofline-ceiling` の -bench 正規表現をアンカー**(`FLOP_AVX2|...$`)。
  従来は `_4acc` にもマッチして4本走っていた(資料は「3つの数字」と説明)。
- **`cmd/isa-report` をビルドタグで分割**(`report_amd64.go` + stub)。
  arm64 で `go test ./...` が isa-report の archsimd import で FAIL していたのを解消。
- 資料(workshop.md)側: 47x/43x の取り違え修正、Xor が整数ベクトル専用である旨の明記、
  持ち帰り節の書き直し(int8 量子化は `DotProductPairs(Saturated)` + `SaturateToInt8` で
  Go 1.26 でも実装可能・Go 1.27 の portable simd / arm64 対応に言及)、
  出力例を実コマンドの出力形式に整合、roofline-batch の数値を本ログ Step 7 の記録値に統一。

## Step 9: 4コア Codespace(参加者と同条件)で全編を再計測(2026-07-07)

SETUP.md は参加者に **4-core** を指定しているのに、資料の実測例は 8コア機の記録だった。
参加者と同じ **standardLinux32gb(4-core / 16GB)** の Codespace(AMD EPYC 7763 / Zen3、
Go 1.26.4、GOEXPERIMENT=simd は devcontainer 済み)で全コマンドを流し、教材の正本数値を
この 4コア実測に置き換えた。devcontainer はそのままで動作、`make test` は初回ビルド込み約10秒。

| 計測 | 結果 |
|---|---|
| `make bench0` SearchNaive | 35.86 ms/op・2.142 GFLOP/s(AI 0.5・153.6 MB/query) |
| `make bench1` DotNaive / DotSIMD | 347.7 ns / 55.22 ns = **6.3x**(かねて資料に載っていた 339/55.9 の出所を今回の記録で確定) |
| `make bench1` SearchSIMD | 7.92 ms・9.70 GF・19.4 GB/s = **4.5x** |
| `make roofline-ceiling` | **25.59 GFLOP/s** / **20.80 read-GB/s** / 17.36 triad-GB/s(4acc は 13.35) |
| `make roofline-batch` | B=1 7.95 ms/q 9.66 GF、BatchNaive 34.24 ms/q 2.243 GF、BatchSIMD 5.79 ms/q 13.26 GF = **5.9x** |
| `make bench2` SearchBinary | 0.768 ms = **46x** |
| `make bench3` SearchBinaryRerank | 0.825 ms = **43x** |
| `make recall` | Recall@10: binary=0.180 binary+rerank=0.868(不変) |
| `make bench-bonus` SearchBinarySIMD | 1.008 ms — VPOPCNT の無い EPYC ではフォールバック分岐のせいで **スカラ SearchBinary(0.77ms)より遅い** |
| `make spill` / `make isa-report` / `make roofline-plot` | いずれも動作確認済み(spill は従来どおり三つ組を表示) |

**変動幅の記録(重要):** 同一マシン・同一コマンドでも共有VMのノイズで
read 帯域は **19.31〜20.80 GB/s**、SearchSIMD は **7.90〜8.91 ms**(9.7〜8.6 GF)まで揺れた。
8コア機の旧記録(read 18.39・達成 18.26 = 99.3%)の「ぴたり一致」は毎回は再現しない。
→ workshop.md に「±5% 揺れる・**9割を超えていれば壁に到達と読む**」の注記を追加し、
資料の主張を揺れに対して頑健な形に直した。

**反映:** workshop.md の数値と図(`rl-stage0〜4` / `roofline-plot` / `memory-vs-compute-roofline` /
`roofline-plot-example`※)を
4コア実測へ更新(リッジ 1.4→1.3、メモリ上限 9.2→≈10 GF、達成 9.7 GF 等)。
Makefile の既定天井を `PEAK=25.59` / `BW=20.80` に変更。8コア表記は撤去し SETUP.md の
4-core 指定と条件を一致させた。
※ roofline-plot-example.png は、記録済みベンチ出力を `cmd/roofline-plot` に食わせて HTML を再生成し、
ヘッドレス Chrome(`--window-size=960,600 --force-device-scale-factor=2`)で撮影。再計測不要で再現できる。

## Step 10: 構成拡張 — 並列・N スイープ・int8・MaxSim を実装して実測(2026-07-08)

レビュー第2弾「もっと打てる手は? もっと SIMD が効く題材は?」への回答として、
本章2つ(寄り道=goroutine 並列、N スイープコラム)と付録2つ(int8、MaxSim)を追加。
計測は前回と同型の 4-core Codespace(EPYC 7763・物理2コア×SMT2・Go 1.26.4)の別インスタンス。
このインスタンスは前回より全体に1割ほど遅かった(共有VMの揺れ。資料の注記どおり)。

### 寄り道: goroutine 並列はどの天井に効くか(make bench-parallel)

| workers | 全探索 B=1(メモリ律速) | バッチ B=32(演算律速) |
|---|---|---|
| 1 | 9.36 ms(16.4 GB/s) | 6.72 ms/query |
| 2 | 5.74 ms(26.8 GB/s)= 1.63x | 4.48 ms/query = 1.50x |
| 4 | 5.20 ms(29.5 GB/s)= **1.80x 頭打ち** | 3.52 ms/query = **1.91x** |

- メモリ律速は合算 ~30 GB/s で**マシン全体の DRAM 帯域**に飽和(2→4 workers で +10% のみ)。
- 演算律速は 1.9x — この 4 vCPU は `lscpu` で **物理2コア × SMT2** と判明。SMT 兄弟は
  実行ユニットを共有するので、演算律速は物理コア数の壁に当たる。
- 教訓: 並列の効きも「何律速か」で決まり、足す前に予測できる。当初の想定
  (演算律速はほぼリニア)は物理4コア機でのみ成立する点に注意 — 資料には
  SMT の壁として正直に記載した。Go カンファレンスで必ず出る「goroutine で
  並列化すれば?」への実測回答。

### N スイープ(make bench-nsweep)

| N | データ量 | naive | SIMD | 倍率 |
|---|---|---|---|---|
| 1,000 | 1.5 MB | 351 µs | 60.9 µs | **5.8x** |
| 10,000 | 15.4 MB | 3.47 ms | 609 µs | **5.7x** |
| 100,000 | 154 MB | 34.8 ms | 8.83 ms | **3.9x** |
| 1,000,000 | 1.5 GB | 347 ms | 89.5 ms | **3.9x** |

L3 に収まる間はカーネル並み(5.7〜5.8x)、DRAM に溢れると 3.9x で一定。
「SIMD が効く境界」がデータサイズ軸にもあることの直接可視化。Stage 1 コラムに採用。

### 付録A: int8 量子化(make bench-int8 / make recall)

- カーネル: DotInt8Naive 364 ns → DotInt8SIMD **34.6 ns = 10.5x**。
  fp32 SIMD(55 ns)より速い — VPMOVSXBW + VPMADDWD で1命令16要素(fp32 の2倍幅)。
  signed×signed なので飽和トリック(VPMADDUBSW 系)が不要になり、コードが素直。
- 全探索: **4.19 ms**(38.4 MB/query・達成 9.2 GB/s)。fp32 SIMD 比 ~2x。
  転送 1/4 なのに 4x にならないのは、壁が遠のいた分カーネルが新しい律速になったため
  (AI = 2 flop/byte でリッジの右)— 「律速は消えず移動する」の追加実例。
- **Recall@10 = 0.948**(binary 0.180 との対比。rerank 不要の実用域)。
  クラスタ合成データは TestRecall と同一生成器。

### 付録B: MaxSim / late interaction(make bench-maxsim)

1万文書 × 4トークン、クエリ16トークン(AI = Tq/2 = 8 flop/byte が方式に内在):
naive 239 ms(2.06 GF)→ SIMD **42 ms(11.7 GF)= 5.7x**。
「バッチ構造が検索方式の仕様として最初から存在する」現代的な題材で、
Stage 2 の正当化に使える。ColBERT / Qdrant マルチベクトルと同系。

### 実装メモ

- SearchParallel / SearchBatchParallel: DB チャンク分割 + worker ローカル topK をマージ
  (チャンクが互いに素なので重複処理不要)。
- bench-bonus の再確認: VPOPCNT 無し機ではフォールバック分岐で SearchBinarySIMD(1.0ms)が
  スカラ(0.77ms)より遅い — 付録Cに「効かない SIMD」の証拠として記載。

## Step 11: int8 を本編 Stage 3 に昇格 — 「バイト削減のエスカレーション」構成へ(2026-07-08)

計測ではなく構成変更。「SIMD が主役で律速を突破し続ける資料」として、②バイト削減を
**int8(1/4・Stage 3)→ 1bit(1/32・Stage 4)の2段のエスカレーション**に再構成した。

- 新番号: Stage 3 = int8、Stage 4 = バイナリ量子化、Stage 5 = rerank。
  付録は A = MaxSim、B = AVX-512 VPOPCNT に再番号。
- 物語上の利得: (1) 量子化の第一歩で SIMD が主役のまま(整数カーネル 10.5x)、
  (2) int8 で「律速がメモリ→カーネルへ移動する」というルーフラインの追加ビート、
  (3) 1bit の Recall 崩壊が「int8 では保てた精度が」という前振り付きで際立つ、
  (4) Stage 5 末尾に fp32 / int8 / 1bit / 1bit+rerank の**速度×精度の設計空間**表。
- 図: rl-stage3(int8・新規、AI=2 でリッジ右の点)を作成し、旧 stage3/4 を
  rl-stage4/5 にリネーム。overview(roofline-plot)に S3 int8 の点を追加。
- Makefile コメント・isa-report(Stage 3 の int8 API 3行を追加)・README・
  コードコメントの番号もすべて追随。make ターゲット名は互換のため変更せず
  (bench2=Stage 4、bench3=Stage 5。対応は workshop.md のコマンド一覧に明記)。

## 高速化の階段(最終形)

> 倍率は **AWS c7i** の史実。Codespaces(EPYC 7763)の実測は Step 7 を参照(SIMD 全探索の倍率が
> 1.6x→4.2x と CPU で変わる)。「絶対値・倍率は約束しない、各自のベンチで点を打つ」が本ワークショップの建付け。

| 段階 | 何をしたか | 1クエリ(c7i) | 倍率(c7i) |
|---|---|---|---|
| Stage 0 | スカラー全探索 | 28.6 ms | 1.0x |
| Stage 1 | 内積を SIMD 化(+VZEROUPPER) | 18.0 ms | 1.6x(カーネル単体は 4.6x) |
| Stage 4 | バイナリ量子化 + スカラー popcount | 0.68 ms | **42x** |
| Stage 5 | + float32 rerank(精度回復 0.18→0.87) | 0.73 ms | **39x** |

> ※ Stage 番号は Step 11 の再構成後のもの(旧 Stage 2=バイナリ、旧 仕上げ=rerank)。
> Stage 2(バッチ)・Stage 3(int8)・寄り道(並列)は後の構成拡張で追加され、
> c7i では未計測(EPYC 4-core の実測は Step 9/10)。

## 付録: 各 Step のコードの場所

文章だけでなく、各段階の実装そのものをコードとして保存してある。
`make remote-steps` で「高速化の階段」を Step 順に再現できる。

| Step | 実装 | 場所 |
|---|---|---|
| Step 0 | `DotNaive` / `Hamming` | `internal/vec/dot.go` / `hamming.go` |
| Step 1 | `dotStep1`(素朴なSIMD、acc1本) | `internal/vec/steps_lab_test.go` |
| Step 2 | `dotStep2`(スライス前進+acc2本、VZなし)/ `hammingStep1` | 同上 |
| Step 3 | `Quantize` / `Hamming` / `SearchBinary*` | `internal/vec` / `internal/index` |
| Step 4 | `dotIdx2` / `dotArr2` / `dotUnsafe2` / `dotUnsafe4` / `dotUnsafeVZ2` | `internal/vec/dot_lab_test.go` |
| Step 4c | `GOAMD64=v3` 実験 | `make remote-dotlab-v3` |
| Step 5 | `Dot` / `HammingSIMD`(VZEROUPPER入り本実装)| `internal/vec/dot_simd.go` 等(当初の自作 `vzeroupper_amd64.s` は Step 8 で `archsimd.ClearAVXUpperBits()` に置換)|

※ 罠③(recall を壊す合成データ)だけはコードを残していない。再現したい場合は
`internal/index/index_test.go` の TestRecall で、センター生成を「正規化してから使う」
に変えると recall が 0.36 程度まで崩れる。

## 付録: 環境にまつわる発見

- **`simd/archsimd` は amd64 専用**(Go 1.26 時点)。arm64(NEON/SVE)は設計段階で未着手。
  教材は `dot_simd.go` / `dot_fallback.go` のビルドタグ分離で将来の arm64 対応に備えてある
- **Apple Silicon + Rosetta では FMA/AVX-512 が使えず、内積 SIMD パスは走らない**。
  [Rosetta は AVX/AVX2 を翻訳するが AVX-512 は非対応](https://developer.apple.com/documentation/apple-silicon/about-the-rosetta-translation-environment)。
  amd64 クロスビルドは通り Rosetta で実行もできるが、実測では `AVX2=true, FMA=false` となり
  `hasSIMD = X86.AVX2() && X86.FMA()` のガードでスカラーにフォールバックする。
  Docker `linux/amd64` も Apple Silicon 上はエミュレーション経由で同種の制約がかかる
- **`MulAdd`(VFMADD213PS)は AVX2 とは別に FMA フィーチャーが必要**。
  ガードは `X86.AVX2() && X86.FMA()` の2段で書く
- **`Uint64x4.OnesCount`(VPOPCNTQ)は AVX512VPOPCNTDQ が必要**。AVX2 のみの CPU には
  SIMD popcount が存在しないため、本編はスカラー `math/bits.OnesCount64` を採用
- **本編で使う SIMD は AVX2 + FMA だけ**(int8 カーネルは AVX2 のみ)。これは過去10年の x86
  (Intel Haswell 2013+ / AMD 2015+)がほぼ全て持つので、**参加者環境は GitHub Codespaces
  一本で全ステージ再現できる**(当たる CPU の Intel/AMD・世代を問わない)。`make bench` がこれ。
- **AVX-512 VPOPCNT(`SearchBinarySIMD`)は本編フロー外の付録に降格**。量子化後はキャッシュ
  律速で popcount を SIMD 化しても速くならない(本ログ Step 5: SearchBinarySIMD 0.75ms ≧
  スカラ SearchBinary 0.68ms)うえ、全 CPU にあるとも限らないため。コードとこの実測は
  証拠として残置(`make bench-bonus`)。
- AWS c7i(Sapphire Rapids / Xeon 8488C)は AVX-512 + VPOPCNTDQ をフル装備。**付録の AVX-512
  を実機で確かめる用**として `infra/` の Terraform 一式 + `make remote-bench` を残す。
  本ログの Step 0〜6 の実測はこの c7i 上の記録(=史実)。**Codespaces(EPYC 7763)での本編
  再計測は実施済み → Step 7**。
