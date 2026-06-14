# devcontainer / Codespaces では go = 1.26 なのでそのまま動く。
# ローカル mac (arm64) では: make GO=$(go env GOPATH)/bin/go1.26.4
GO ?= go
export GOEXPERIMENT = simd

.PHONY: test bench bench0 bench1 bench2 bench3 bench-bonus roofline roofline-batch roofline-ceiling roofline-decompose recall cpuinfo isa-report

test:
	$(GO) test ./...

## Stage 0: スカラー全探索(ベースライン)
bench0:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchNaive$$' -benchtime 2s

## Stage 1: SIMD 内積
bench1:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD)$$' -benchtime 2s

## Stage 2: バイナリ量子化
bench2:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary)$$' -benchtime 2s

## 仕上げ: Stage 0/1/2 + float32 rerank(本編の最終形。AVX2+FMA だけで完結)
bench3:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary|BinaryRerank)$$' -benchtime 2s

bench: bench3

## (付録) AVX-512 VPOPCNT で popcount を SIMD 化。Stage 2 で見たとおり量子化後はキャッシュ
## 律速で速くならない(SearchBinarySIMD ≧ SearchBinary)ことの確認用。本編フロー外。
## AVX-512 + VPOPCNTDQ 機(AWS c7i 等)以外ではスカラにフォールバックする。
bench-bonus:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearchBinarySIMD$$' -benchtime 2s

## ルーフライン: 各 Stage の GFLOP/s・AI・MB/query を表示して図に「点を打つ」
## (Stage 0 naive → 1 SIMD → 2 binary の3点。docs/workshop/workshop.md 参照)
roofline:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(Naive|SIMD|Binary)$$' -benchtime 2s

## バッチ化の効き: B=1(全探索) vs B=32(バッチ)で scalar/SIMD を比較
## 演算律速にすると SIMD が exact 検索でも効くことを見る(docs/workshop/workshop.md Stage 2)
roofline-batch:
	$(GO) test ./internal/index -run - -bench 'BenchmarkSearch(SIMD|BatchNaive|BatchSIMD)$$' -benchtime 2s

## ルーフラインの天井そのものを実測: 演算ピーク(FMA飽和) + メモリ帯域(read/triad)
## これで推定だった天井を実測値へ置き換える(docs/workshop/workshop.md §04)
roofline-ceiling:
	$(GO) test ./internal/vec -run - -bench 'BenchmarkPeak(FLOP|ReadBW|TriadBW)' -benchtime 2s

## 「メモリ時間 vs 演算時間」の反転図を、実測天井から再生成(docs/workshop §06 Stage 2)
## 自分のマシンの天井で: make roofline-decompose PEAK=<GF> BW=<GB/s> (天井は make roofline-ceiling)
## PNG 化には rsvg-convert が要る(無ければ SVG だけ更新)。
PEAK ?= 25.51
BW   ?= 18.39
roofline-decompose:
	$(GO) run ./cmd/roofline-decompose -peak $(PEAK) -bw $(BW) > docs/images/memory-vs-compute-roofline.svg
	@command -v rsvg-convert >/dev/null 2>&1 \
	  && rsvg-convert -w 1920 docs/images/memory-vs-compute-roofline.svg -o docs/images/memory-vs-compute-roofline.png \
	  || echo "(PNG はスキップ: rsvg-convert が無い)"

## Recall@10 の計測(binary vs binary+rerank)
recall:
	$(GO) test ./internal/index -run TestRecall -v

## この CPU で使える SIMD 機能を表示
cpuinfo:
	$(GO) test ./internal/vec -run TestDotMatchesNaive -v | grep -E 'HasSIMD|ok|FAIL'

## 各 Stage の archsimd API と archsimd.X86 対応を一覧 (pkg.go.dev 準拠)
isa-report:
	GOARCH=amd64 GOEXPERIMENT=simd $(GO) run ./cmd/isa-report/

# ---- リモート実行 (infra/ の AVX-512 VM) ----------------------------------
# 使い方: cd infra && terraform apply してから make remote-bench

REMOTE_HOST ?= $(shell terraform -chdir=infra output -raw public_ip 2>/dev/null)
SSH_OPTS    := -o StrictHostKeyChecking=accept-new
REMOTE      := ubuntu@$(REMOTE_HOST)
REMOTE_DIR  := simd-search
REMOTE_RUN  = ssh $(SSH_OPTS) $(REMOTE) 'cd $(REMOTE_DIR) && GOEXPERIMENT=simd go

.PHONY: remote-sync remote-test remote-bench remote-roofline remote-roofline-batch remote-roofline-ceiling remote-recall remote-cpuinfo

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
	$(REMOTE_RUN) test ./internal/vec -run - -bench "BenchmarkPeak(FLOP|ReadBW|TriadBW)" -benchtime 2s'

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
