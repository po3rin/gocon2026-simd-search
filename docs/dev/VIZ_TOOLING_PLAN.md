# 可視化・プロファイル ツール導入プラン

ワークショップ運営用メモ。現状の workshop は「**Go 標準ベンチだけで GFLOP/s・AI・天井を出す**」+
「ルーフラインは静止画 SVG(`docs/images/rl-stage*.png`)」で完結している。
ここに **ビジュアル/対話的な確認手段**を足し、各章の主張を「推定」から「画面で見える証拠」へ格上げする。

## 方針(2つだけ)

1. **workshop の思想を壊さない** — 主役は Go 標準同梱 or Go エコシステムのツール。
   全員が Codespaces / AWS c7i で同じ手順で動かせること。特別なプロファイラの常用は前提にしない。
2. **Go エンジニアが今後も使える道具に寄せる** — pprof / `go tool objdump` / `GOSSAFUNC` /
   `golang.org/x/perf` / go-echarts など、ワークショップ後も実務で再利用できるものを選ぶ。
   非 Go ツール(llvm-mca, uiCA, perf, toplev)は「深掘り・講師デモ限定」に留める。

> **環境の前提(重要)**: HW カウンタ(perf / toplev / LIKWID / pcm)は通常の AWS VM では
> PMU が制限され数字が崩れる(§10 の Docker 罠と同根)。**本プランの 3 施策はすべて PMU 不要**で、
> Codespaces を含め全員が動かせるものだけで構成する。計器系は別途「講師デモ(\*.metal)」として切り出す。

---

## 施策1: `make profile` — pprof でフレームグラフ & 逐次 disassembly(最優先・全員配布)

### 狙い(どの章で何を確認するか)
- **§06 Stage0**: `sum += ...` の依存連鎖律速 = 命令並列ゼロを、disassembly の命令時間で見る。
- **§06 Stage1**: カーネルは速いのに全探索が伸びない = 時間が **load に乗っている**(メモリ待ち)を flame graph で見る。
- **§06 Stage2**: バッチ化で時間が `vec.Dot`(演算)へ移るのを flame graph で見る。
- **§12 VZEROUPPER / spill**: 水平和の SSE 命令・FMA 周辺の load/store に時間が乗るのを disassembly で見る。

### なぜこのツールか(Go エンジニア向き)
`go tool pprof` は **Go 標準同梱**。`-http` でブラウザに flame graph / call graph /
source 注釈 / **逐次 disassembly(命令ごとのサンプル数)** が出る。追加依存ゼロ。
ワークショップ後もあらゆる Go コードの性能調査でそのまま使える。

### 実装案
- `Makefile` に `profile` ターゲットを追加(Stage を引数で選べる形に):
  ```makefile
  ## 指定 Stage の CPU プロファイルを取り、pprof をブラウザで開く
  ## 例: make profile BENCH=BenchmarkSearchSIMD
  BENCH ?= BenchmarkSearchSIMD
  profile:
  	$(GO) test ./internal/index -run - -bench '$(BENCH)$$' -benchtime 2s -cpuprofile /tmp/cpu.out
  	$(GO) tool pprof -http=:8080 /tmp/cpu.out
  ```
- 既存の `internal/index/bench_test.go` はそのまま使える(`-cpuprofile` はテスト側の標準フラグ)。
- `remote-profile`(infra の c7i 上で取得 → ローカルへ scp → pprof)も同型で足せる。

### workshop への差し込み
- §06 各 Stage と §12 に「**画面で確かめる**」小コラムを1つずつ。
  「`make profile BENCH=...` → flame で時間がどこか / disassembly で命令時間」を1〜2行。
- §12 は disassembly view のスクショを `docs/images/` に1枚足すと、現状の objdump テキストが「時間付き」に強化される。

### 検証 / 注意
- `go tool pprof` の disassembly も objdump 同様に VEX 系(FMA)を誤訳しうる。**バイト列で正体を確認**する注記は §12 と同じものを流用。
- サンプリングなので 1 回の実行ではブレる。`-benchtime` を伸ばすか複数回。

### 工数感: 小(Makefile 数行 + workshop コラム + スクショ1枚)

---

## 施策2: §12(spill / VZEROUPPER)の codegen 可視化 — Go ネイティブを主役に

### 狙い(どの章で何を確認するか)
- **§12 隠れ天井②(register spill)**: FMA ごとに load+store が付く = レジスタに収まっていないことを可視化。
- **§12 隠れ天井①(VZEROUPPER 税)**: SIMD→スカラ境界(水平和)で何が出ているかを可視化。

### なぜこのツールか(Go エンジニア向き)
非 Go の llvm-mca / uiCA は「ポート圧・サイクル」まで見えるが Python/LLVM 依存。
**まず Go 標準/Go 対応で完結**させ、深掘りだけ外部ツールに回す:

| ツール | 種別 | 何が見える | 位置づけ |
|---|---|---|---|
| **`GOSSAFUNC=Dot go build`** | Go 標準 | コンパイラの SSA と **regalloc パス**を `ssa.html` で対話表示。spill が起きる過程そのもの | 主役(§12②に直結) |
| **`go tool objdump -s Dot`** | Go 標準 | 最終機械語。load/store ペアと FMA を確認(現状の手法) | 主役(既存) |
| **pprof disassembly**(施策1) | Go 標準 | 上記に「命令ごとの時間」を重ねる | 主役 |
| **Compiler Explorer (godbolt)** | Go 対応・セルフホスト可 OSS | source↔asm を色で対応。誤訳せず `VFMADD…` 表示 | 任意(公開鯖は `GOEXPERIMENT=simd` 不可の可能性→セルフホスト) |
| **llvm-mca / uiCA** | 非 Go | 内側ループの **ポート圧・throughput・critical path**(0.65 FMA/cyc の裏取り) | 深掘り任意。objdump で抜いた asm を貼る |

### 実装案
- `Makefile` に補助ターゲット:
  ```makefile
  ## Dot の SSA/regalloc を ssa.html に出力(register spill の可視化, §12)
  ssa:
  	GOSSAFUNC=Dot GOEXPERIMENT=simd $(GO) build ./internal/vec/ || true
  	@echo "open ./ssa.html"

  ## Dot の最終機械語(load/store と FMA を確認)
  disasm:
  	$(GO) test -c -o /tmp/vec.test ./internal/vec
  	$(GO) tool objdump -s 'Dot$$' /tmp/vec.test
  ```
- llvm-mca 用には「objdump から内側ループを切り出す手順」を `docs/dev/` の手順メモに1ブロック(任意・深掘り)。

### workshop への差し込み
- §12②の objdump 例の直後に「`make ssa` で regalloc を見る」を1行追加(現状のテキスト説明を対話 HTML で補強)。
- 「ポート圧まで見たい人へ」として llvm-mca/uiCA を**任意の脚注**に。本編 40 分は Go ネイティブだけで完結させる。

### 検証 / 注意
- `GOSSAFUNC` の出力関数名は実際のシンボルに合わせる(`Dot` / `DotNaive` 等)。ビルドが通らなくても `ssa.html` は出る(`|| true`)。
- §12 の「どこまで確かか(正直に)」の姿勢を維持 — spill の**存在**は可視化できるが「消せば理論ピーク」は依然未検証、と明記。

### 工数感: 小〜中(Makefile + workshop 脚注。godbolt セルフホストまでやるなら別途)

---

## 施策3: `make roofline-plot` — 実測値から対話的ルーフライン HTML を生成

### 狙い(どの章で何を確認するか)
- **§04〜§06 全体**: 今は静止画の `rl-stage*.png`。**`make roofline` を叩くたびに点が動く**体験にし、
  「測る → 点が打たれる → 天井に当たる」をその場で見せる。

### なぜこのツールか(Go エンジニア向き)
作図も **Pure Go** で閉じる:

| ツール | 役割 | Go 再利用性 |
|---|---|---|
| **`golang.org/x/perf/benchfmt`** | `go test -bench` 出力(独自単位 AI・GFLOP/s 含む)を解析 | Go 公式。ベンチ解析の正攻法。実務で再利用可 |
| **`go-echarts`**(`github.com/go-echarts/go-echarts/v2`) | 散布図+屋根の線を**対話 HTML(ECharts)**で出力 | Pure Go。ダッシュボード作成で実務再利用可 |
| (代替)**`gonum/plot`** | 静止 SVG/PNG を生成 | Pure Go。`rl-stage*.svg` を実測から**自動再生成**する用途にも使える |

`bench_test.go` は既に `ReportMetric` で `AI(flop/byte)` と `GFLOP/s` を出している。
`benchfmt` はこの**カスタム単位をそのまま読める**ので、追加計装は不要。

### 実装案
- 新規 `cmd/roofline-plot/main.go`:
  1. 標準入力 or ファイルから `go test -bench` 出力を `benchfmt.NewReader` で読む。
  2. 各ベンチの `AI` と `GFLOP/s` を点に、`roofline-ceiling` の天井(39/120/240・帯域 11)を屋根の線に。
  3. go-echarts で log-log 散布図 + 折れ線 → `roofline.html` を出力。
- `Makefile`:
  ```makefile
  ## 実測値から対話的ルーフライン HTML を生成して開く
  roofline-plot:
  	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary)$$' -benchtime 2s \
  	  | $(GO) run ./cmd/roofline-plot > /tmp/roofline.html
  	@echo "open /tmp/roofline.html"
  ```
- go.mod に `golang.org/x/perf` と `go-echarts/v2` を追加(どちらも Pure Go、cgo なし)。

### workshop への差し込み
- §06 冒頭の全体図(`roofline-plot.png`)の下に「`make roofline-plot` で**自分の実測の対話版**が出る」を1行。
- 余力があれば `gonum/plot` 版で `rl-stage*.svg` を実測から再生成し、画像と数字の正本を一致させる(memory: 改善過程は再現可能に)。

### 検証 / 注意
- ECharts の log 軸 + 屋根の線(斜線→水平線の折れ)は custom option が要る場合あり。まず点だけ→屋根を足す順で。
- 天井の数値は `roofline-ceiling` の実測に追従させる(ハードコードせず、可能なら同時に読む)。

### 工数感: 中(小さな Go プログラム1本 + 依存2つ + workshop 1行)

---

## 導入順(おすすめ)

1. **施策1(pprof / `make profile`)** — 効果最大・工数最小・全員即動く。まずこれ。
2. **施策3(`make roofline-plot`)** — 「測ると点が動く」で workshop の主役図を強化。
3. **施策2(`make ssa` / `make disasm`)** — §12 深掘り。本編必須ではないので最後。

## あえて入れないもの(理由)

- **perf / toplev(TMA) / LIKWID / pcm-memory** … メモリ律速の計器証拠としては最強だが、
  通常 VM では PMU 制限で数字が崩れる。使うなら **\*.metal の講師デモ限定**。全員配布には載せない。
- **pyroscope / Grafana(continuous profiling)** … 40 分には過剰。実務向けの紹介に留める。

---

参照: 各章の主張は [workshop.md](../workshop/workshop.md)、実測の生ログは [OPTIMIZATION_LOG.md](OPTIMIZATION_LOG.md)。
