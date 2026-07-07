# gocon2026-simd-search

Go Conference 2026 ショートワークショップ
**「ルーフラインで読み解く Pure Go × SIMD ベクトル検索」** の教材リポジトリ。

> 🚧 現在は CFP 応募用の実測フェーズ。ワークショップ教材は **[`docs/workshop/workshop.md`](docs/workshop/workshop.md)**(読んで動かす形式・穴埋めなし)。

## 何をするか

Go 1.26 の実験的 SIMD パッケージ(`GOEXPERIMENT=simd` / `simd/archsimd`)を使い、
外部ライブラリなしの Pure Go でベクトル検索エンジンを高速化する。ただし闇雲に触らず、
**ルーフラインモデル**という一枚の地図の上で、毎回この4手を回す:

> **① 測る → ② 算術強度(AI)を出して図に点を打つ → ③ 当たっている天井を特定 → ④ その天井を狙う手だけ打つ**

ルーフラインは、**まだ1行も最適化していない段階で「縦に上っても天井で頭打ち、
本命は AI を右に動かすこと」を予言してくれる**。その地図に沿って実装するのが本教材。

| Stage | 内容 | ルーフライン上の動き | カーネル |
|---|---|---|---|
| 0 | スカラー全探索(ベースライン) | AI 0.5・どの天井にも未達 | `vec.DotNaive` |
| 1 | 内積の SIMD 化(`Float32x8` + FMA) | **縦に上る** → メモリ斜線に張り付く | `vec.Dot` |
| 2 | バイナリ量子化 + ハミング距離(byte 1/32) | **横に動く** → DRAM 律速を脱出 | `vec.Quantize` + `vec.Hamming` |
| 仕上げ | binary で粗く絞って float32 で rerank | 精度軸(Recall@10 0.18→0.87) | `Index.SearchBinaryRerank` |

> 本編で使う SIMD は **AVX2 + FMA だけ**(Stage 1 と仕上げの内積)。だから **GitHub Codespaces にどの CPU が当たっても全ステージ再現します**。AVX-512 VPOPCNT は Stage 2 で見たとおり量子化後は速くならないので本編では扱いません(AVX-512 機で試したい人向けの付録 `vec.HammingSIMD` のみ残置)。

中核メッセージ: **SIMD だけが高速化じゃない。ルーフラインで天井を見れば、
「縦に上る(実装効率)」と「横に動く(データ表現)」のどちらを打つべきかが図から決まる。
そして横に動いた先でまた SIMD が効く。**

ワークショップの進め方・本物のルーフライン(Codespaces / AMD EPYC 7763 実測)・各 Stage の点と天井・計測方法・原典は、
教材 **[`docs/workshop/workshop.md`](docs/workshop/workshop.md)** に集約(各 Stage の点と当たっている天井を静止画のルーフラインで示し、Go コードと計測コマンドを併記)。

## 動かし方

### Codespaces / devcontainer(推奨)

`.devcontainer/` に Go 1.26 + `GOEXPERIMENT=simd` 環境を定義済み。開いてそのまま:

```sh
make test       # 正しさの確認
make roofline   # 各 Stage の GFLOP/s・AI・MB/query を表示して「図に点を打つ」
make bench      # 本編フル(スカラ/SIMD/バイナリ/rerank)。AVX2+FMA だけで完結
make recall     # Recall@10(binary vs rerank vs int8)
make bench-parallel # 寄り道: goroutine 並列はどの天井に効くか
make bench-nsweep   # Stage 1 コラム: DB サイズで SIMD 倍率が崩れる境界
make bench-int8     # Stage 3: int8 量子化(カーネル 10x・Recall 0.948)
make bench-maxsim   # 付録A: MaxSim(late interaction・最初から演算律速)
make bench-bonus    # 付録B: AVX-512 VPOPCNT。AVX-512機向け・速くならない確認用
```

`make roofline` の出力例(点を打つ = ルーフラインの①②):

```
# 例: 4コア Codespace(AMD EPYC 7763)。数値は当たった CPU・実行ごとの揺れで変わります
BenchmarkSearchNaive   ... ns/op   ... MB/s   0.5 AI(flop/byte)   2.15 GFLOP/s   153.6 MB/query
BenchmarkSearchSIMD    ... ns/op   ... MB/s   0.5 AI(flop/byte)   9.7 GFLOP/s    153.6 MB/query  ← メモリ斜線(read天井)に張り付く
BenchmarkSearchBinary  ... ns/op   ... MB/s                                       4.8 MB/query  ← 横に動いて 1/32
```

### ローカル(mac / arm64)

`simd/archsimd` は amd64 専用なので、arm64 ではスカラーフォールバックでビルド・テストだけ通る:

```sh
go install golang.org/dl/go1.26.4@latest && go1.26.4 download
make GO=$(go env GOPATH)/bin/go1.26.4 test
```

amd64 クロスビルド(Rosetta 実行)で SIMD パスのコンパイル確認も可能:

```sh
GOARCH=amd64 GOEXPERIMENT=simd go1.26.4 test ./...
```

各 Stage の `archsimd` API が要求する CPU 機能と、`archsimd.X86` でこの CPU が対応しているかを一覧([pkg.go.dev/simd/archsimd](https://pkg.go.dev/simd/archsimd) 準拠):

```sh
make isa-report GO=$(go env GOPATH)/bin/go1.26.4
```

## 構成

```
internal/vec/    距離カーネル(ワークショップで穴埋めする場所)
internal/index/  ミニ検索エンジン(Index / Search API)+ ベンチ + roofline 計測
docs/workshop/   参加者教材 workshop.md(SIMD/ベクトル検索の基礎+進め方+図+計測方法+まとめ+原典)
docs/dev/        開発記録(OPTIMIZATION_LOG / ENVIRONMENT_SURVEY)
docs/images/     図(SVG+PNG)
docs/README.md   ドキュメント索引
PROPOSAL.md      CFP プロポーザル
```
