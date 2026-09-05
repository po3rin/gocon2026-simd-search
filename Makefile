# devcontainer / Codespaces では go = 1.27 なのでそのまま動く。
# ローカル mac (arm64) では: make GO=$(go env GOPATH)/bin/go1.27.1
# (Go 1.27 から archsimd が arm64 Neon に対応したので、Mac でも SIMD パスが走る)
GO ?= go
export GOEXPERIMENT = simd

.PHONY: test bench bench0 bench1 bench2 bench3 bench-portable bench-bonus bench-parallel bench-nsweep bench-int8 bench-maxsim recall-int8 roofline roofline-batch roofline-ceiling roofline-decompose roofline-plot spill recall cpuinfo isa-report isa-report-amd64

test:
	$(GO) test ./...

## Stage 0: スカラー全探索(ベースライン)
bench0:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchNaive$$' -benchtime 2s

## Stage 1: SIMD 内積(カーネル単体 + 全探索の2粒度。workshop.md Stage 1 の 6.3x / 4.5x を再現)
bench1:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkDot(Naive|SIMD)$$' -benchtime 2s
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD)$$' -benchtime 2s

## Stage 1 コラム: ポータブル simd パッケージ(Go 1.27 の simd.Float32s)。
## archsimd 版と同じ内積をベクトル長非依存で書いたもの(vec.DotPortable)。
## 3行目は GODEBUG=simd=128 でレジスタ幅を半分(AVX2 機なら 256→128bit)にして同じ全探索を測る。
## 256bit は archsimd 版と同じ点(壁)、128bit はカーネルが 1 ベクトルのメモリ時間からはみ出して
## 壁の下に落ちる = 「幅は広げても壁の上に行けず、狭めると下に落ちる」(workshop.md Stage 1 コラム)。
## arm64(Neon)は元から 128bit なので 2行目と 3行目は同じ数字になる。
bench-portable:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkDot(Naive|SIMD|Portable)$$' -benchtime 2s
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(SIMD|Portable)$$' -benchtime 2s
	GODEBUG=simd=128 $(GO) test ./internal/index -run - -bench 'BenchmarkSearchPortable$$' -benchtime 2s

## Stage 4: バイナリ量子化(1bit・1/32)
bench2:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary)$$' -benchtime 2s

## Stage 5(仕上げ): スカラ/SIMD/バイナリ + float32 rerank(本編の最終形。AVX2+FMA だけで完結)
bench3:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary|BinaryRerank)$$' -benchtime 2s

bench: bench3

## (付録B) AVX-512 VPOPCNT で popcount を SIMD 化。Stage 4 で見たとおり量子化後はキャッシュ
## 律速で速くならない(SearchBinarySIMD ≧ SearchBinary)ことの確認用。本編フロー外。
## AVX-512 + VPOPCNTDQ 機(AWS c7i 等)以外ではスカラにフォールバックする。
bench-bonus:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchBinarySIMD$$' -benchtime 2s

## 寄り道: goroutine 並列はどの天井に効くか(workshop.md 寄り道節)。
## メモリ律速の全探索(B=1)はコアが DRAM 帯域を取り合うのでサブリニア、
## 演算律速のバッチ(B=32)はほぼリニアに伸びる。
bench-parallel:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Parallel|BatchParallel)$$' -benchtime 2s

## N スイープ(workshop.md Stage 1 コラム): DB サイズを 1k→1M と振り、
## キャッシュに収まる間は SIMD が効き、DRAM に溢れると倍率が崩れるのを見る。
## 1M の index 構築(数秒)が初回に走る。
bench-nsweep:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchSweep$$' -benchtime 1s -timeout 30m

## Stage 3: int8 量子化(1/4 サイズ)。カーネル(VPMOVSXBW+VPMADDWD)と全探索。
## 精度は make recall(TestRecallInt8 も走る)で確認。
bench-int8:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkDotInt8(Naive|SIMD)$$' -benchtime 2s
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchInt8$$' -benchtime 2s

## Stage 3 の精度: int8 単体の Recall@10(本編の進行用 — binary/rerank の行は Stage 4/5 で見る)
recall-int8:
	$(GO) test ./internal/index -run 'TestRecallInt8$$' -v

## 付録A: MaxSim(late interaction)。1ロードに多数の内積がタスクに内在
## = 最初から演算律速で、SIMD が最初から効く検索方式。
bench-maxsim:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchMaxSim(Naive|SIMD)$$' -benchtime 2s

## ルーフライン: 各 Stage の GFLOP/s・AI・MB/query を表示して図に「点を打つ」
## (Stage 0 naive → 1 SIMD → 4 binary の3点。docs/workshop/workshop.md 参照)
roofline:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary)$$' -benchtime 2s

## バッチ化の効き: B=1(全探索) vs B=32(バッチ)で scalar/SIMD を比較
## 演算律速にすると SIMD が exact 検索でも効くことを見る(docs/workshop/workshop.md Stage 2)
roofline-batch:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(SIMD|BatchNaive|BatchSIMD)$$' -benchtime 2s

## ルーフラインの天井そのものを実測: 演算ピーク(FMA飽和) + メモリ帯域(read/triad)
## これで推定だった天井を実測値へ置き換える(docs/workshop/workshop.md §05)
## amd64 は FLOP_AVX2、arm64(Apple Silicon)は FLOP_NEON が走る(ReadBW/TriadBW は arm64 ではスカラ版)。
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

## 実測値から対話的ルーフライン HTML を生成(docs/workshop §06)。叩くたびに点が打たれ、
## Stage 1(AI=0.5)はメモリ壁に張り付き、Stage 2 バッチ(AI=16)はリッジを越えて演算側へ動く。
## 自分のマシンの天井で: make roofline-plot PEAK=<GF> BW=<GB/s> (天井は make roofline-ceiling)
## 理論ピークの線も出すなら TPEAK=100(AVX2 理論値・CPU 依存なので既定は off)。
## ※ amd64 (Codespaces/AWS) で実行して初めて意味のある数字になる。arm64 はスカラ退避で潰れる。
TPEAK ?= 0
roofline-plot:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|BatchNaive|BatchSIMD)$$' -benchtime 2s \
	  | $(GO) run ./cmd/roofline-plot -peak $(PEAK) -bw $(BW) -tpeak $(TPEAK) > /tmp/roofline.html
	@echo "open /tmp/roofline.html"

## register spill を見る(docs/workshop §05)。演算ピーク(12本アキュムレータ)ループ
## BenchmarkPeakFLOP_AVX2 の機械語をコンパイラ -S で出し、各アキュムレータ aN が毎回
## 「ロード(SP)→VFMADD→ストア(SP)」とスタックへ退避(spill)している様子を表示する。
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

# ---- リモート実行 (infra/ の AVX-512 VM) ----------------------------------
# 使い方: cd infra && terraform apply してから make remote-bench

REMOTE_HOST ?= $(shell terraform -chdir=infra output -raw public_ip 2>/dev/null)
SSH_OPTS    := -o StrictHostKeyChecking=accept-new
REMOTE      := ubuntu@$(REMOTE_HOST)
REMOTE_DIR  := simd-search
REMOTE_RUN  = ssh $(SSH_OPTS) $(REMOTE) 'cd $(REMOTE_DIR) && GOEXPERIMENT=simd go

.PHONY: remote-sync remote-test remote-bench remote-roofline remote-roofline-batch remote-roofline-ceiling remote-roofline-plot remote-recall remote-cpuinfo remote-dotlab remote-dotlab-v3 remote-steps

remote-sync:
	@test -n "$(REMOTE_HOST)" || (echo "VM がない: cd infra && terraform apply" && exit 1)
	rsync -az --delete -e "ssh $(SSH_OPTS)" --exclude .git --exclude infra ./ $(REMOTE):$(REMOTE_DIR)/

remote-test: remote-sync
	$(REMOTE_RUN) test ./...'

## AVX-512 VM で全ステージのベンチを実行(カーネル単体 + 全探索)
remote-bench: remote-sync
	$(REMOTE_RUN) test ./internal/... -run - -bench Benchmark -benchtime 2s'

## AVX-512 VM でルーフラインの3点(GFLOP/s・AI・MB/query)を計測
remote-roofline: remote-sync
	$(REMOTE_RUN) test ./internal/index -run - -bench "BenchmarkSearch(Naive|SIMD|Binary)$$" -benchtime 2s'

## AVX-512 VM でバッチ化の効き(B=1 vs B=32, scalar vs SIMD)を実測
remote-roofline-batch: remote-sync
	$(REMOTE_RUN) test ./internal/index -run - -bench "BenchmarkSearch(SIMD|BatchNaive|BatchSIMD)$$" -benchtime 2s'

## AVX-512 VM で天井そのもの(演算ピーク + メモリ帯域)を実測
remote-roofline-ceiling: remote-sync
	$(REMOTE_RUN) test ./internal/vec -run - -bench "BenchmarkPeak(FLOP_AVX2|ReadBW|TriadBW)$$" -benchtime 2s'

## AVX-512 VM の実測でルーフライン HTML を生成(描画はローカルで)。天井は PEAK/BW で渡す。
remote-roofline-plot: remote-sync
	$(REMOTE_RUN) test ./internal/index -run - -bench "BenchmarkSearch(Naive|SIMD|BatchNaive|BatchSIMD)$$" -benchtime 2s' \
	  | $(GO) run ./cmd/roofline-plot -peak $(PEAK) -bw $(BW) -tpeak $(TPEAK) > /tmp/roofline.html
	@echo "open /tmp/roofline.html"

remote-recall: remote-sync
	$(REMOTE_RUN) test ./internal/index -run TestRecall -v'

## Dot 実装のコード生成比較ラボ
remote-dotlab: remote-sync
	$(REMOTE_RUN) test ./internal/vec -run TestDotVariants -v -bench "BenchmarkDot" -benchtime 2s'

## OPTIMIZATION_LOG.md の「高速化の階段」を Step 順に再現
remote-steps: remote-sync
	$(REMOTE_RUN) test ./internal/vec -run TestStepsMatchNaive -v -bench BenchmarkStep -benchtime 2s'

## 同上 + GOAMD64=v3(スカラーもVEXエンコードにしてSSE/AVX混在を消す実験)
remote-dotlab-v3: remote-sync
	ssh $(SSH_OPTS) $(REMOTE) 'cd $(REMOTE_DIR) && GOEXPERIMENT=simd GOAMD64=v3 go test ./internal/vec -run TestDotVariants -bench "BenchmarkDot" -benchtime 2s'

## VM の CPU で AVX2 / AVX-512 が引けているか確認
remote-cpuinfo: remote-sync
	$(REMOTE_RUN) test ./internal/vec -run TestDotMatchesNaive -v' | grep -E 'HasSIMD|ok'
