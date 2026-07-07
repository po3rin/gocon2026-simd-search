# Go の SIMD の2つの隠れ天井（VZEROUPPER 税 / register spill）

> これは深掘りメモです。ワークショップ本編（[`../workshop/workshop.md`](../workshop/workshop.md)）の
> Stage 0〜4 とは独立した読み物で、**Go 1.26 archsimd のコード生成の今**に踏み込みたい人向け。
> ハードの限界ではなく **Go のコード生成がまだ若い**ことの表れで、2つは**同根**です。

> ⚠️ **計測機について:** この調査は **AWS c7i(Intel Xeon 8488C / Sapphire Rapids）** で行ったものです。
> ワークショップ本編は GitHub Codespaces（当たる CPU は AMD EPYC 7763 など）で動かすため、数値の桁が違います。
> 重要な差:
> - **① VZEROUPPER 税は Intel 固有**で、**AMD（Zen）では基本的に出ません**（Zen には Intel 型の遷移ペナルティが無い）。
>   むしろ VZEROUPPER 自体が高コストな世代もあります。本実装が境界で `archsimd.ClearAVXUpperBits()`(= VZEROUPPER)を呼ぶのは Intel 機での保険で、AMD では無害なだけ。
> - **② register spill は CPU を問わず再現**します。Codespaces の EPYC でも `4本=13.4GF < 12本=25.5GF`（本数を減らすと遅くなる）という同じ指紋が出ます。
>
> 生ログは [`OPTIMIZATION_LOG.md`](OPTIMIZATION_LOG.md)（VZEROUPPER は Step 4、register spill は Step 6/6b）。

## 隠れ天井①: VZEROUPPER 税 — SIMD→スカラ境界の遷移ペナルティ

**症状:** 内積カーネルを SIMD 化したのに、全探索の**見かけの帯域が妙に低い(5.7 GB/s)**。カーネル単体も 167ns で頭打ち。ところが境界に**たった1命令 `VZEROUPPER` を置くだけで 167→23ns = 7.1x** 速くなり、帯域の見かけの壁も消えました。

![VZEROUPPER 税のあり/なし比較](../images/vzeroupper.png)

図: 同じコードでも、SIMD→スカラ境界に `VZEROUPPER` を1命令置くかどうかで 167ns→23ns(7.1x)。左(税あり)は dirty な YMM 上位とレガシー SSE の衝突でペナルティが命令ごとに蓄積し、右(税なし)は `VZEROUPPER` で上位128bitを掃除して衝突そのものを消す。

**なぜ起きるか:** AVX2 命令(`Float32x8` の FMA など)を使うと YMM レジスタの**上位128bitが「dirty(汚れた)」状態**になります。その直後、水平和の `sum = buf[0]+buf[1]+…` は**スカラの float32 計算**で、Go はこれを**レガシー SSE 命令**で出力します。この「dirty な YMM 上位 × レガシー SSE」の組み合わせが CPU にペナルティを発生させる。`VZEROUPPER` は上位128bitをゼロに掃除する1命令で、境界で一度呼べばこの税金が消えます(命令自体のコストは小さく、差し引きで大きく得をする — ただし後述のとおりベンダー差はある)。

**ペナルティの正体(世代で違う・ここ重要):** 教科書でよく語られる「**一度きりの大きな遷移ペナルティ(上位状態をセーブ/リストアするモード切替)**」は Sandy/Ivy Bridge〜Haswell 世代の挙動です。**Skylake 以降の modern Intel では仕組みが変わり**、もう上位状態を保存せず、dirty 状態で実行する**レガシー SSE 命令1個ごとに false dependency(上位ビットへの偽の依存)+ マージ(blend)μop が挿入される**形になっています。テスト機 `AWS c7i`(4th Gen Xeon Scalable = Sapphire Rapids)はまさにこの後者で、実測した「呼び出しごとの固定費 ≒145ns(550cyc)」は、この per-instruction ペナルティの蓄積を VZEROUPPER がまとめて消していると読むのが正確です。

**Go 固有の事情:** Go 1.26 の archsimd は**この VZEROUPPER を自動挿入しません**。本来コンパイラが境界を管理して入れてくれることを期待したいところで、実際 [golang/go#77647](https://github.com/golang/go/issues/77647) でも「intrinsics はコンパイラ管理だから VEX 遷移は面倒を見てくれるはず…?」という**未解決の問い**として挙がっています。現状、境界に何も置かないと生成コードに VZEROUPPER は1個も出ません。回避策は標準 API の **`archsimd.ClearAVXUpperBits()`**(中身は VZEROUPPER 1命令。doc コメントにも「将来コンパイラが自動生成するかもしれない」と明記)を境界で呼ぶこと。本リポも当初は3行の自作アセンブリ `vzeroupper_amd64.s` を使っていたが、この標準 API に置き換えた。下の register spill と**同根のコード生成の未熟さ**で、これも将来 Go 側で解消される見込みです。

**どこまで確かか(正直に):** 「VZEROUPPER 1命令で同一コードが 7倍速くなった」は実測で確認済み(167→23ns)。ただし**効果の大きさはベンダー・世代依存**で、ここの値は modern Intel(Sapphire Rapids)のもの。**AMD Zen には Intel 型の遷移ペナルティが基本的に無く**、むしろ VZEROUPPER 自体が高コストな世代もある(=同じ7倍は出ない)。また 550cyc の固定費を命令単位まで分解したわけではなく、**dim スケーリングで固定費を分離 → VZEROUPPER 投入で7倍を確認**、という状況証拠による特定です。生ログは [`OPTIMIZATION_LOG.md`](OPTIMIZATION_LOG.md) の Step 4。

## 隠れ天井②: なぜ FMA は AVX2ピークの約1/3か — register spill

演算天井(AVX2)は理論 **~120 GFLOP/s**(2 FMA/cyc × 8レーン × 2flop × 3.75GHz)。実測は **39 GF = その約1/3(33%、0.65 FMA/cyc)**(c7i)。AI=0.5 の深いメモリ律速なのでこの低さは検索の結論を変えませんが、原因は面白いところです。`objdump` で内側ループを見ると、独立なはずのアキュムレータ12本(4本版でも同様)が**全部レジスタに置けず、毎回スタックへ退避(register spill)**され、同じ1本のレジスタを使い回していました。**FMA 1個ごとに load+store が必ず付く**ので、(1) load/store ポートが先に飽和し、(2) 次の周回の load が今回の store を待つ(store-to-load forwarding 〜5〜7cyc)。アキュムレータを増やすほど隠れるので **4本 < 12本**(c7i は 23GF < 39GF、Codespaces の EPYC でも 13.4GF < 25.5GF)と増えます — 「待ち時間律速」の指紋です。これを図にすると次のとおりです。

![register spill: 理想(レジスタ常駐)vs 実際(スタック往復)](../images/register-spill.png)

図: 理想はアキュムレータをレジスタに置いたまま回す。実際は毎回スタックへ退避(register spill)し、FMA ごとに load+store が付く。

register spill = レジスタに収まらない/置けない値をメモリ(スタック)へ追い出すこと。これはハードの限界ではなく **Go 1.26 archsimd のレジスタ割り当ての未熟さ**(VZEROUPPER を自動挿入しないのと同根のコード生成課題)。既知 issue [golang/go#76969](https://github.com/golang/go/issues/76969) と同件で、レジスタ割り当ての改善は [#78753](https://github.com/golang/go/issues/78753) など **Go 1.27 で進行中** — **将来このピークは上がる見込み**。生ログは [`OPTIMIZATION_LOG.md`](OPTIMIZATION_LOG.md) の Step 6/6b。

**どこまで確かか(正直に):** 確認できたのは **「spill が存在する」**(objdump で 4本・12本とも FMA に load+store が付く ＋ 上流 issue [#76969](https://github.com/golang/go/issues/76969))と、メモリポート律速の見積り(〜0.6 FMA/cyc)が実測 0.65 とほぼ一致する点まで。**「spill さえ消せば理論ピークに届く」は未検証** — Go 1.26 archsimd は常に spill し Go コードでは消せないため、VZEROUPPER のような「1命令足したら7倍」の決定的な介入実験ができていません。よって本節は**強い状況証拠による推定**であって断定ではありません。

```text
// go tool objdump で見た内側ループ(アキュムレータ1本ぶん)
0x..e3   c5fe6f9424...   VMOVDQU 0x398(SP), X2   ← スタックから レジスタへ load
0x..74   c4e27da8d1      TESTL $0xd1, AL         ← 実は VFMADD213PS(=FMA)。go の誤訳
0x..79   c5fe7f9424...   VMOVDQU X2, 0x398(SP)   ← レジスタから スタックへ store
```

**objdump の読み方(3点だけ):** ①1行は「アドレス / 命令バイト列 / ニーモニック(命令名) オペランド」。`(SP)` はスタック上の場所。②`VMOVDQU`=ベクトルのコピー(メモリ⇄レジスタ)、`VFMADD…PS`=FMA。③**落とし穴**: `go tool objdump` は新しめの命令(VEX系)を誤訳し、FMA が `TESTL $0xd1, AL` のような化けで表示されます — **正体は左のバイト列**(`c4e2…`)を見れば分かります。Linux の `objdump -d` なら正しく表示されます。

**スピルの見分け方:** ループ内で**演算ごとに**「`(SP)→レジスタ` の load」と「`レジスタ→(SP)` の store」がセットで並んでいたら、値をレジスタに保持できず毎回スタックを往復している証拠 = register spill。理想は load/store が消えて**FMA だけが並ぶ**。
