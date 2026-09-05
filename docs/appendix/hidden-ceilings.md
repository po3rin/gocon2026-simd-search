# 付録: Go の SIMD の 2 つの隠れた性能上限(VZEROUPPER の遷移ペナルティと register spill)

> 本編の 40 分には入らない深掘りです。ワークショップ本編（[`../workshop/workshop.md`](../workshop/workshop.md)）の
> Stage 0〜4 とは独立した読み物で、Go 1.26 / 1.27 archsimd のコード生成の現状に踏み込みたい人向け。
> どちらもハードの限界ではなく、Go のコード生成がまだ発展途上であることの表れで、原因は共通です。

> ⚠️ **計測機について:** この調査は **AWS c7i(Intel Xeon 8488C / Sapphire Rapids）** で行ったものです。
> ワークショップ本編は GitHub Codespaces（当たる CPU は AMD EPYC 7763 など）で動かすため、数値の桁が違います。
> 重要な差:
> - ① VZEROUPPER の遷移ペナルティは Intel 固有で、AMD（Zen）では基本的に出ません（Zen には Intel 型の遷移ペナルティが無い）。
>   むしろ VZEROUPPER 自体が高コストな世代もあります。本実装が境界で `archsimd.ClearAVXUpperBits()`(= VZEROUPPER)を呼ぶのは Intel 機での保険で、AMD では無害なだけ。
> - ② register spill は CPU を問わず再現します。Codespaces の EPYC でも `4本=13.4GF < 12本=25.5GF`（本数を減らすと遅くなる）という同じ傾向が出ます。
>
> 生ログは [optimization-log.md](optimization-log.md)（VZEROUPPER は Step 4、register spill は Step 6/6b）。

## 隠れた上限①: VZEROUPPER の遷移ペナルティ（SIMD からスカラへ戻る境界で起きる）

**症状:** 内積カーネルを SIMD 化したのに、全探索の見かけの帯域が低い(5.7 GB/s)。カーネル単体も 167ns で頭打ち。境界に 1 命令 `VZEROUPPER` を置くだけで 167ns から 23ns(7.1x)に速くなり、見かけの帯域の上限も消えました。

![VZEROUPPER のあり/なし比較](../images/vzeroupper.png)

図: 同じコードでも、SIMD からスカラへ戻る境界に `VZEROUPPER` を 1 命令置くかどうかで 167ns と 23ns(7.1x)。左(VZEROUPPER なし)は dirty な YMM 上位とレガシー SSE の衝突でペナルティが命令ごとに蓄積し、右(VZEROUPPER あり)は `VZEROUPPER` で上位128bitを掃除して衝突そのものを消す。

**なぜ起きるか:** AVX2 命令(`Float32x8` の FMA など)を使うと YMM レジスタの上位 128bit が「dirty(汚れた)」状態になります。その直後、水平和の `sum = buf[0]+buf[1]+…` はスカラの float32 計算で、Go はこれをレガシー SSE 命令で出力します。この「dirty な YMM 上位 × レガシー SSE」の組み合わせが CPU にペナルティを発生させます。`VZEROUPPER` は上位 128bit をゼロに掃除する 1 命令で、境界で一度呼べばこのペナルティが消えます(命令自体のコストは小さく、差し引きで大きく得をします。ただし後述のとおりベンダー差があります)。

**ペナルティの中身(世代で違う):** 教科書でよく語られる「一度きりの大きな遷移ペナルティ(上位状態をセーブ/リストアするモード切替)」は Sandy/Ivy Bridge〜Haswell 世代の挙動です。Skylake 以降の Intel では仕組みが変わり、上位状態を保存せず、dirty 状態で実行するレガシー SSE 命令 1 個ごとに false dependency(上位ビットへの偽の依存)とマージ(blend)μop が挿入される形になっています。テスト機 `AWS c7i`(4th Gen Xeon Scalable = Sapphire Rapids)はまさにこの後者で、実測した「呼び出しごとの固定費 ≒145ns(550cyc)」は、この per-instruction ペナルティの蓄積を VZEROUPPER がまとめて消していると読むのが正確です。

**Go 固有の事情:** Go の archsimd はこの VZEROUPPER を自動挿入しません(1.26・1.27 とも。1.27 のコンパイラにも VZEROUPPER を出す経路は `ClearAVXUpperBits` のイントリンシックだけ)。本来はコンパイラが境界を管理して入れるべきもので、[golang/go#77647](https://github.com/golang/go/issues/77647) でも「intrinsics はコンパイラ管理だから VEX 遷移は面倒を見てくれるのでは」という未解決の問いとして挙がっています。現状、境界に何も置かないと生成コードに VZEROUPPER は 1 個も出ません。回避策は標準 API の `archsimd.ClearAVXUpperBits()`(中身は VZEROUPPER 1 命令。doc コメントにも「将来コンパイラが自動生成するかもしれない」と明記)を境界で呼ぶことです。本リポも当初は 3 行の自作アセンブリ `vzeroupper_amd64.s` を使っていましたが、この標準 API に置き換えました。下の register spill と原因は共通で、将来 Go 側で解消される見込みです。

**どこまで確かか:** 「VZEROUPPER 1 命令で同一コードが 7 倍速くなった」は実測で確認済みです(167ns から 23ns)。ただし効果の大きさはベンダー・世代依存で、ここの値は Sapphire Rapids のものです。AMD Zen には Intel 型の遷移ペナルティが基本的に無く、むしろ VZEROUPPER 自体が高コストな世代もあります(同じ 7 倍は出ません)。また 550cyc の固定費を命令単位まで分解したわけではなく、dim スケーリングで固定費を分離し、VZEROUPPER 投入で 7 倍を確認した、という状況証拠による特定です。生ログは [optimization-log.md](optimization-log.md) の Step 4。

## 隠れた上限②: なぜ FMA は AVX2 ピークの約 1/3 か（register spill）

演算ピーク(AVX2)は理論 約 120 GFLOP/s(2 FMA/cyc × 8 レーン × 2flop × 3.75GHz)。実測は 39 GF で、その約 1/3(33%、0.65 FMA/cyc)です(c7i)。AI=0.5 の深いメモリ律速なのでこの低さは検索の結論を変えませんが、原因は見ておく価値があります。`objdump` で内側ループを見ると、独立に持てるアキュムレータ 12 本(4 本版でも同様)が全部レジスタに置けず、毎回スタックへ退避(register spill)され、同じ 1 本のレジスタを使い回していました。FMA 1 個ごとに load+store が必ず付くので、(1) load/store ポートが先に飽和し、(2) 次の周回の load が今回の store を待ちます(store-to-load forwarding 5〜7cyc)。アキュムレータを増やすほど待ち時間が隠れるので、4 本より 12 本の方が速くなります(c7i は 23GF と 39GF、Codespaces の EPYC でも 13.4GF と 25.5GF)。待ち時間で律速しているときの典型的な傾向です。これを図にすると次のとおりです。

![register spill: 理想(レジスタ常駐)vs 実際(スタック往復)](../images/register-spill.png)

図: 理想はアキュムレータをレジスタに置いたまま回す。実際は毎回スタックへ退避(register spill)し、FMA ごとに load+store が付く。

register spill とは、レジスタに収まらない、または置けない値をメモリ(スタック)へ追い出すことです。これはハードの限界ではなく、Go の archsimd のレジスタ割り当てが発展途上であることによるものです(VZEROUPPER を自動挿入しないのと共通のコード生成の課題)。既知 issue [golang/go#76969](https://github.com/golang/go/issues/76969)(closed / not planned)と同件。[#78753](https://github.com/golang/go/issues/78753)(AVX-512 の上位 16 本の ZMM が割り当てられない件)は Go 1.27 で閉じたが、**この 12 本 AVX2 ループの spill は Go 1.27.1 でも残っている**(`make spill GO=go1.27.1` で 48 行の `VMOVDQU …(SP)`)。arm64(Neon)でも同じで、M3 Pro の 12 本 FMLA ループは `FMOVQ …(SP)` に挟まれて 32 GFLOP/s 止まり。将来このピークは上がる見込みですが、1.27 ではまだです。生ログは [optimization-log.md](optimization-log.md) の Step 6/6b。

**どこまで確かか:** 確認できたのは「spill が存在する」こと(objdump で 4 本・12 本とも FMA に load+store が付く。上流 issue [#76969](https://github.com/golang/go/issues/76969))と、メモリポート律速の見積り(約 0.6 FMA/cyc)が実測 0.65 とほぼ一致することまでです。「spill さえ消せば理論ピークに届く」は未検証です。Go 1.26 / 1.27 の archsimd は常に spill し、Go コードでは消せないため、VZEROUPPER のような「1 命令足したら 7 倍」の決定的な介入実験ができていません。本節は状況証拠による推定であって、断定ではありません。

```text
// go tool objdump で見た内側ループ(アキュムレータ1本ぶん)
0x..e3   c5fe6f9424...   VMOVDQU 0x398(SP), X2   ← スタックから レジスタへ load
0x..74   c4e27da8d1      TESTL $0xd1, AL         ← 実は VFMADD213PS(=FMA)。go の誤訳
0x..79   c5fe7f9424...   VMOVDQU X2, 0x398(SP)   ← レジスタから スタックへ store
```

**objdump の読み方(3点だけ):** ①1行は「アドレス / 命令バイト列 / ニーモニック(命令名) オペランド」。`(SP)` はスタック上の場所。②`VMOVDQU`=ベクトルのコピー(メモリ⇄レジスタ)、`VFMADD…PS`=FMA。③注意: `go tool objdump` は新しめの命令(VEX 系)を誤訳し、FMA が `TESTL $0xd1, AL` のような化けで表示されます。本当の命令は左のバイト列(`c4e2…`)を見れば分かります。Linux の `objdump -d` なら正しく表示されます。

**spill の見分け方:** ループ内で演算ごとに「`(SP)` からレジスタへの load」と「レジスタから `(SP)` への store」がセットで並んでいたら、値をレジスタに保持できず毎回スタックを往復している証拠です(register spill)。理想は load/store が消えて FMA だけが並ぶ状態です。
