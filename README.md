# gocon2026-simd-search

Go Conference 2026 ショートワークショップ
**「ルーフラインで読み解く Pure Go × SIMD ベクトル検索」** の教材リポジトリ。

## 参加者の方へ

必要なのは GitHub アカウントとブラウザだけです。

1. **環境を用意する**: [docs/workshop/SETUP.md](docs/workshop/SETUP.md) の手順で GitHub Codespaces を起動します(`Code` → `Codespaces` → `Create codespace`、マシンは 4-core)。ブラウザで VS Code が開けば準備完了です
2. **動作確認**: Codespace のターミナルで `make test` と `make bench0` を実行します
3. **教材を読みながら進める**: [docs/workshop/workshop.md](docs/workshop/workshop.md) を上から順に読み、各 Stage の `make` コマンドを実行して自分の数字を見ます

手元の PC で動かす場合(amd64 Linux / Windows、Apple Silicon の Mac)の手順は [この下の「動かし方」](#動かし方) にあります。

## 何をするか

Go 1.27 の実験的 SIMD パッケージ(`GOEXPERIMENT=simd` / `simd/archsimd`)を使い、
外部ライブラリなしの Pure Go でベクトル検索エンジンを高速化する。ただし闇雲には触らず、
ルーフラインモデル(性能の上限を図にする方法。教材 §04 で説明)を使って毎回この 4 手を回す:

1. 測る
2. 算術強度(AI)を出して図に点を打つ
3. いま何が性能の上限になっているかを特定する
4. その上限に効く手だけ打つ

ルーフラインを使うと、1 行も最適化していない段階で「実装を速くしてもメモリ帯域で頭打ちになる。
本命は AI を上げること」が分かる。その見立てに沿って実装するのが本教材。

| Stage | 内容 | ルーフライン上の動き | カーネル |
|---|---|---|---|
| 0 | スカラー全探索(ベースライン) | AI 0.5・どの天井にも未達 | `vec.DotNaive` |
| 1 | 内積の SIMD 化(`Float32x8` + FMA) | **縦に上る** → メモリ斜線に張り付く | `vec.Dot` |
| 2 | クエリのバッチ化(B=32・exact) | **横に動く**(AI 16)→ リッジを越えて演算側 | `vec.Dot`(DB ロードを再利用) |
| コラム | goroutine 並列(workers=1/2/4) | 別の上限(マシン全体の帯域 / 物理コア数)に当たる | `Index.SearchParallel` |
| 3 | int8 量子化(byte 1/4) | **右へ**(AI 2)→ リッジ越え・カーネル律速へ | `vec.QuantizeInt8` + `vec.DotInt8` |
| 4 | バイナリ量子化 + ハミング距離(byte 1/32) | **右上へ** → DRAM 律速を脱出(Recall 0.18) | `vec.Quantize` + `vec.Hamming` |
| 5 | binary で粗く絞って float32 で rerank | 精度軸(Recall@10 0.18→0.87) | `Index.SearchBinaryRerank` |

> 本編で使う SIMD は AVX2 + FMA だけ(Stage 3 の int8 カーネルは AVX2 のみ)。そのため GitHub Codespaces にどの CPU が当たっても全ステージ再現する。AVX-512 VPOPCNT は Stage 4 で見るとおり量子化後は速くならないので本編では扱わない(AVX-512 機で試したい人向けの付録 `vec.HammingSIMD` のみ残している)。

伝えたいことは 1 つ。SIMD だけが高速化ではない。ルーフラインで性能の上限を見れば、
実装効率を上げる(SIMD)のか、データ表現を変える(バッチ化・量子化)のか、どちらを打つべきかが図から決まる。
そしてデータ表現を変えた先でまた SIMD が効く。

ワークショップの進め方、Codespaces(AMD EPYC 7763)で実測したルーフライン、各 Stage の位置、計測方法、原典は
教材 [`docs/workshop/workshop.md`](docs/workshop/workshop.md) にまとめてある(各 Stage の位置を図で示し、Go コードと計測コマンドを併記)。
ドキュメントの一覧は [`docs/README.md`](docs/README.md)。

## 動かし方

### Codespaces / devcontainer(推奨)

`.devcontainer/` に Go 1.27 + `GOEXPERIMENT=simd` 環境を定義済み。開いてそのまま:

```sh
make test       # 正しさの確認
make roofline   # 各 Stage の GFLOP/s・AI・MB/query を表示して「図に点を打つ」
make bench      # 本編フル(スカラ/SIMD/バイナリ/rerank)。AVX2+FMA だけで完結
make recall     # Recall@10(binary vs rerank vs int8)
make bench-parallel # コラム: goroutine 並列はどの上限に効くか
make bench-nsweep   # Stage 1 コラム: DB サイズで SIMD 倍率が崩れる境界
make bench-int8     # Stage 3: int8 量子化(カーネル 10x・Recall 0.948)
make bench-portable # Stage 1 コラム: ポータブル simd 版(256bit で同じ結果)+ GODEBUG=simd=128 で幅を半分にすると遅くなる
make bench-maxsim   # 付録A: MaxSim(late interaction・最初から演算律速)
make bench-bonus    # 付録B: AVX-512 VPOPCNT。AVX-512機向け・速くならない確認用
```

`make roofline` の出力例(上の 4 手の 1〜2 にあたる):

```
# 例: 4コア Codespace(AMD EPYC 7763)。数値は当たった CPU・実行ごとの揺れで変わります
BenchmarkSearchNaive   ... ns/op   ... MB/s   0.5 AI(flop/byte)   2.15 GFLOP/s   153.6 MB/query
BenchmarkSearchSIMD    ... ns/op   ... MB/s   0.5 AI(flop/byte)   9.7 GFLOP/s    153.6 MB/query  ← メモリ帯域の上限(read 約 20 GB/s)に達する
BenchmarkSearchBinary  ... ns/op   ... MB/s                                       4.8 MB/query  ← 転送量を 1/32 に減らす
```

### ローカル(mac / arm64)

Go 1.27 から `simd/archsimd` が arm64(Neon・128bit)に対応したので、Apple Silicon でも SIMD パスが走る
(`internal/vec/dot_arm64.go` / `int8_arm64.go`。天井ベンチも Neon 版あり):

```sh
go install golang.org/dl/go1.27.1@latest && go1.27.1 download
make GO=$(go env GOPATH)/bin/go1.27.1 test
make GO=$(go env GOPATH)/bin/go1.27.1 bench1   # Neon 版の数字(本編の AVX2 とは別物。workshop.md §09)
```

amd64 クロスビルド(Rosetta 実行)で amd64 側の SIMD パスのコンパイル確認も可能(Rosetta は FMA 非対応なので実行はスカラに落ちる):

```sh
GOARCH=amd64 GOEXPERIMENT=simd go1.27.1 test ./...
```

各 Stage の `archsimd` API が要求する CPU 機能と、この CPU が対応しているかを一覧([pkg.go.dev/simd/archsimd](https://pkg.go.dev/simd/archsimd) 準拠。arm64 では Neon 版カーネルの一覧が出る):

```sh
make isa-report GO=$(go env GOPATH)/bin/go1.27.1
make isa-report-amd64 GO=$(go env GOPATH)/bin/go1.27.1   # Rosetta で amd64 側の一覧
```

## 構成

```
internal/vec/    距離カーネル(Stage ごとの内積・ハミング距離の実装)
internal/index/  ミニ検索エンジン(Index / Search API)+ ベンチ + roofline 計測
docs/workshop/   参加者教材 workshop.md(SIMD/ベクトル検索の基礎+進め方+図+計測方法+まとめ+原典)
docs/appendix/   付録(隠れた性能上限 / 実行環境の調査 / 最適化の記録)
docs/images/     図(SVG+PNG)
docs/README.md   ドキュメント索引
```

## ライセンス

MIT License。コード、教材、図のすべてに適用します([LICENSE](LICENSE))。
