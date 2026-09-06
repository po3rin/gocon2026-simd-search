# devcontainer / Codespaces では go = 1.27 なのでそのまま動く。
# ローカル mac (arm64) では: make GO=$(go env GOPATH)/bin/go1.27.1
# (Go 1.27 から archsimd が arm64 Neon に対応したので、Mac でも SIMD パスが走る)
GO ?= go
export GOEXPERIMENT = simd

.PHONY: test lint fmt bench bench0 bench1 bench2 bench3 bench-portable bench-bonus bench-parallel bench-nsweep bench-int8 bench-maxsim recall-int8 roofline roofline-batch roofline-ceiling roofline-decompose roofline-figures roofline-plot spill recall cpuinfo isa-report isa-report-amd64

test:
	$(GO) test ./...

## lint: golangci-lint(.golangci.yml)を amd64 と arm64 の両方で回す(CI と同じ)。
## golangci-lint は Go 1.27 でビルドされた v2.13 以上が要る: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
GOLANGCI ?= golangci-lint
lint:
	GOARCH=amd64 $(GOLANGCI) run ./...
	GOARCH=arm64 $(GOLANGCI) run ./...

## fmt: gofmt を全ファイルに適用(CI は差分が無いことだけ確認する)
fmt:
	$(GO) fmt ./...

## Stage 0: スカラー全探索(ベースライン)
bench0:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchNaive$$' -benchtime 2s

## Stage 1: SIMD 内積(カーネル単体 + 全探索の2粒度。workshop.md Stage 1 の 6.3x / 4.5x を再現)
bench1:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkDot(Naive|SIMD)$$' -benchtime 2s
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD)$$' -benchtime 2s

## Stage 1 コラム: ポータブル simd パッケージ(Go 1.27 の simd.Float32s)で書いた同じ内積(vec.DotPortable)。
## 3、4 行目は GODEBUG=simd=128 でレジスタ幅を半分(AVX2 機なら 256bit から 128bit)にして同じ全探索とカーネルを測る。
## 256bit は archsimd 版と同じ点(メモリ帯域の上限)に乗り、128bit はカーネルが遅くなって上限の下に落ちる
## (workshop.md Stage 1 コラム)。arm64(Neon)は元から 128bit なので 2 行目と 3 行目は同じ数字になる。
bench-portable:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkDot(Naive|SIMD|Portable)$$' -benchtime 2s
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(SIMD|Portable)$$' -benchtime 2s
	GODEBUG=simd=128 $(GO) test ./internal/index -run - -bench 'BenchmarkSearchPortable$$' -benchtime 2s
	GODEBUG=simd=128 $(GO) test ./internal/vec -run - -bench 'BenchmarkDotPortable$$' -benchtime 2s

## Stage 4: バイナリ量子化(1bit・1/32)
bench2:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary)$$' -benchtime 2s

## Stage 5(仕上げ): スカラ/SIMD/バイナリ + float32 rerank(本編の最終形。AVX2+FMA だけで完結)
bench3:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary|BinaryRerank)$$' -benchtime 2s

bench: bench3

## (付録 appendix.md 3 節) AVX-512 VPOPCNT で popcount を SIMD 化。量子化後はキャッシュ律速で速くならない
## (SearchBinarySIMD ≧ SearchBinary)ことの確認用。本編フロー外。AVX-512 + VPOPCNTDQ のある機械(AWS c7i 等を
## 各自で用意)以外ではスカラにフォールバックする。
bench-bonus:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchBinarySIMD$$' -benchtime 2s

## コラム: goroutine 並列はどの上限に効くか(workshop.md §06 のコラム)。
## メモリ律速の全探索(B=1)はコアが DRAM 帯域を取り合うのでサブリニア、
## 演算律速のバッチ(B=32)はほぼリニアに伸びる。
bench-parallel:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Parallel|BatchParallel)$$' -benchtime 2s

## N スイープ(workshop.md Stage 1 コラム): DB サイズを 1k から 1M まで振り、
## キャッシュに収まる間は SIMD が効き、DRAM に溢れると倍率が崩れるのを見る。
## 1M の index 構築(数秒)が初回に走る。
bench-nsweep:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchSweep$$' -benchtime 1s -timeout 30m

## Stage 3: int8 量子化(1/4 サイズ)。カーネル(VPMOVSXBW+VPMADDWD)と全探索。
## 精度は make recall(TestRecallInt8 も走る)で確認。
bench-int8:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkDotInt8(Naive|SIMD)$$' -benchtime 2s
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchInt8$$' -benchtime 2s

## Stage 3 の精度: int8 単体の Recall@10(binary と rerank の行は Stage 4/5 で見る)
recall-int8:
	$(GO) test ./internal/index -run 'TestRecallInt8$$' -v

## 付録 appendix.md 2 節: MaxSim(late interaction)。1 回のロードに多数の内積が最初から含まれるので、
## 最初から演算律速で SIMD が効く検索方式。
bench-maxsim:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchMaxSim(Naive|SIMD)$$' -benchtime 2s

## ルーフライン: 各 Stage の GFLOP/s・AI・MB/query を表示して図に「点を打つ」
## (Stage 0 naive、Stage 1 SIMD、Stage 4 binary の 3 点。docs/workshop/workshop.md 参照)
roofline:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary)$$' -benchtime 2s

## バッチ化の効き: B=1(全探索) vs B=32(バッチ)で scalar/SIMD を比較
## 演算律速にすると SIMD が exact 検索でも効くことを見る(docs/workshop/workshop.md Stage 2)
roofline-batch:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(SIMD|BatchNaive|BatchSIMD)$$' -benchtime 2s

## ルーフラインの天井そのものを実測: 演算ピーク(FMA飽和) + メモリ帯域(read/triad)
## これで推定だった天井を実測値へ置き換える(docs/workshop/workshop.md §05)
## amd64 は FLOP_AVX2、arm64(Apple Silicon)は FLOP_NEON が走る(ReadBW/TriadBW も arm64 は Neon 版)。
roofline-ceiling:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkPeak(FLOP_AVX2|FLOP_NEON|ReadBW|TriadBW)$$' -benchtime 2s

## 「メモリ時間 vs 演算時間」の反転図を、実測天井から再生成(docs/workshop §06 Stage 2)
## 自分のマシンの天井で: make roofline-decompose PEAK=<GF> BW=<GB/s> (天井は make roofline-ceiling)
## PNG 化には rsvg-convert が要る(無ければ SVG だけ更新)。
PEAK ?= 25.59
BW   ?= 20.80
roofline-decompose:
	$(GO) run ./cmd/roofline-decompose -peak $(PEAK) -bw $(BW) > docs/images/memory-vs-compute-roofline.svg
	@command -v rsvg-convert >/dev/null 2>&1 \
	  && rsvg-convert -w 1920 docs/images/memory-vs-compute-roofline.svg -o docs/images/memory-vs-compute-roofline.png \
	  || echo "(PNG はスキップ: rsvg-convert が無い)"

## 静止画のルーフライン図(docs/images/roofline-concept, rl-stage0〜5, roofline-plot)を同じ見た目で再生成。
## 数値は cmd/roofline-figures/main.go に直書き(workshop.md の Codespaces 実測値)。再計測したらそこを直して叩く。
## PNG 化には rsvg-convert が要る(無ければ SVG だけ更新)。
roofline-figures:
	$(GO) run ./cmd/roofline-figures -peak $(PEAK) -bw $(BW) -out docs/images
	@command -v rsvg-convert >/dev/null 2>&1 \
	  && for f in roofline-concept rl-stage0 rl-stage1 rl-stage2 rl-stage3 rl-stage4 rl-stage5 roofline-plot; do \
	       rsvg-convert -w 1920 docs/images/$$f.svg -o docs/images/$$f.png; done \
	  || echo "(PNG はスキップ: rsvg-convert が無い)"

## 実測値から対話的ルーフライン HTML を生成(docs/workshop §06)。叩くたびに点が打たれ、
## Stage 1(AI=0.5)はメモリ帯域の上限に張り付き、Stage 2 バッチ(AI=16)はリッジを越えて演算側へ動く。
## 自分のマシンの天井で: make roofline-plot PEAK=<GF> BW=<GB/s> (天井は make roofline-ceiling)
## 理論ピークの線も出すなら TPEAK=100(AVX2 理論値・CPU 依存なので既定は off)。
## ※ 本編の数字は amd64(Codespaces)のもの。arm64(Neon)でも動くが別の数字になる。
TPEAK ?= 0
roofline-plot:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|BatchNaive|BatchSIMD)$$' -benchtime 2s \
	  | $(GO) run ./cmd/roofline-plot -peak $(PEAK) -bw $(BW) -tpeak $(TPEAK) > /tmp/roofline.html
	@echo "open /tmp/roofline.html"

## register spill を見る(docs/workshop §05)。演算ピーク(12本アキュムレータ)ループ
## BenchmarkPeakFLOP_AVX2 の機械語をコンパイラ -S で出し、各アキュムレータ aN が毎回
## 「ロード(SP)、VFMADD、ストア(SP)」とスタックへ退避(spill)している様子を表示する。
## 12本+m+c=14 は使える 15本の Y レジスタ(Y15 は Go ABI の予約ゼロレジスタ:
## golang/go#76969)に収まる数なので、本数圧ではなく Go のコード生成の問題(1.26 / 1.27 とも退避する)。
## amd64 用にクロスコンパイルするので mac でも可(objdump と違い -S は VFMADD を正名で出す)。
spill:
	GOARCH=amd64 $(GO) test -gcflags=-S -c -o /dev/null ./internal/vec 2>&1 \
	  | awk '/\tTEXT\t.*BenchmarkPeakFLOP_AVX2\(SB\)/{f=1;next} /\tTEXT\t/{f=0} f' \
	  | grep -E 'VFMADD|a[0-9]+\+[0-9]+\(SP\)' \
	  | sed -E 's#\(/[^)]*\)##; s#github\.com/[^ ]*/internal/vec\.##g'

## Recall@10 の計測(binary / binary+rerank / int8)
recall:
	$(GO) test ./internal/index -run TestRecall -v

## この CPU で使える SIMD 機能を表示
cpuinfo:
	$(GO) test ./internal/vec -run TestDotMatchesNaive -v | grep -E 'HasSIMD|ok|FAIL'

## 各 Stage の archsimd API と CPU 機能の対応を一覧 (pkg.go.dev 準拠)。
## amd64 なら archsimd.X86.* のチェック結果、arm64(Apple Silicon)なら Neon 版カーネルの一覧が出る。
## Mac から amd64 側の一覧を見たいときは: make isa-report-amd64 (Rosetta 実行・FMA=false になる)
isa-report:
	GOEXPERIMENT=simd $(GO) run ./cmd/isa-report/

isa-report-amd64:
	GOARCH=amd64 GOEXPERIMENT=simd $(GO) run ./cmd/isa-report/
