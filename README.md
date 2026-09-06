# gocon2026-simd-search

Go Conference 2026 ショートワークショップ [「Go × SIMDで高速化するベクトル検索 ~ ルーフラインモデルでSIMDが効く境界を探れ！ ~」](https://gocon.jp/2026/timetable/1264338/) の教材リポジトリ。

Go 1.27 の実験的 SIMD パッケージ(`simd/archsimd`)で Pure Go のベクトル検索を高速化する。ただし闇雲には触らず、ルーフラインモデルで「いま何が性能の上限か」を見てから手を打つ。SIMD 化、バッチ化、int8 量子化、バイナリ量子化と rerank を順に試し、それぞれがルーフライン上でどう動くかを自分の数字で確かめる。

## はじめ方

必要なのは GitHub アカウントとブラウザだけです。

1. [docs/workshop/setup.md](docs/workshop/setup.md) の手順で GitHub Codespaces を起動する(マシンは 4-core)
2. ターミナルで `make test` と `make bench0` を実行して動作確認する
3. 教材 [docs/workshop/workshop.md](docs/workshop/workshop.md) を上から順に読み、各 Stage の `make` コマンドで自分の数字を見る

手元の Mac(Apple Silicon)で動かす手順も教材の §03 にあります。

## ドキュメント

- [docs/workshop/workshop.md](docs/workshop/workshop.md): 教材本体。ルーフラインの進め方、各 Stage の図とコードと計測コマンド、原典
- [docs/appendix/README.md](docs/appendix/README.md): 付録。Go の SIMD の隠れた性能上限、MaxSim、AVX-512 popcount、実行環境の調査
- [docs/README.md](docs/README.md): ドキュメント索引

## 構成

```
internal/vec/    距離カーネル(Stage ごとの内積・ハミング距離の実装)
internal/index/  ミニ検索エンジン(Index / Search API)+ ベンチ + roofline 計測
cmd/             計測補助ツール(isa-report、roofline の分解・作図)
docs/            教材・付録・図
Makefile         各 Stage の計測コマンド(コメント付き)
```

## ライセンス

MIT License。コード、教材、図のすべてに適用します([LICENSE](LICENSE))。
