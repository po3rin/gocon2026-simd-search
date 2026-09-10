![Go × SIMDで高速化するベクトル検索 ~ ルーフラインモデルでSIMDが効く境界を探れ！ ~(Go Conference 2026 WorkshopB)](docs/images/workshop-hero.png)

# gocon2026-simd-search

Go Conference 2026 ショートワークショップ [「Go × SIMDで高速化するベクトル検索 ~ ルーフラインモデルでSIMDが効く境界を探れ！ ~」](https://gocon.jp/2026/timetable/1264338/) の教材リポジトリ。

Go 1.27 の実験的 SIMD パッケージ(`simd/archsimd`)で Pure Go のベクトル検索を高速化する。ルーフラインモデルで「いま何が性能の上限か」を見ながら、SIMD 化、量子化、rerank などを試しながら高速化していく。

## はじめ方

必要なのは GitHub アカウントとブラウザだけです。

1. [docs/workshop/setup.md](docs/workshop/setup.md) の手順で GitHub Codespaces を起動する
2. [docs/workshop/workshop.md](docs/workshop/workshop.md) を上から順に読み、コードを確認しながらコマンドを実行していく。

## ドキュメント

- [docs/workshop/workshop.md](docs/workshop/workshop.md): ワークショップ教材
- [docs/appendix/appendix.md](docs/appendix/appendix.md): 付録
- [docs/README.md](docs/README.md): ドキュメント索引

## スライド

- [https://speakerdeck.com/po3rin/gocon2026-workshop](https://speakerdeck.com/po3rin/gocon2026-workshop)

## 構成

```
internal/vec/    距離カーネル(Stage ごとの内積・ハミング距離の実装)
internal/index/  ミニ検索エンジン(Index / Search API)+ ベンチ + roofline 計測
cmd/             計測補助ツール(isa-report、roofline の作図)
docs/            教材・付録・図
Makefile         各 Stage の計測コマンド(コメント付き)
```

## ライセンス

MIT License。コード、教材、図のすべてに適用します([LICENSE](LICENSE))。
