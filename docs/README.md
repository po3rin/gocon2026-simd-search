# docs: ドキュメント索引

## ワークショップ向け

- [workshop/setup.md](workshop/setup.md): まずここ。Codespaces の起動、マシンサイズ、費用、困ったときの対処
- [workshop/workshop.md](workshop/workshop.md): 教材本体。SIMD とベクトル検索の基礎、ルーフラインの進め方(測る、算術強度を出す、上限を見る、上限に効く手を打つ)、各 Stage の図とコードと計測コマンド、原典。本編は GitHub Codespaces(AMD EPYC 7763 で実測)前提

数値の正本はこの教材と、`make roofline` / `make roofline-ceiling` の実測です。

## 登壇スライド

- [slides/part1.html](slides/part1.html): 前半(§01〜§05)の講義パート、15 分ぶん・22 枚。ブラウザで開いて ← → でめくる(`f` で全画面、Cmd+P で PDF 出力)。図は `docs/images/*.svg` を参照しているので、図を直したらスライドにも反映される

## 付録

- [appendix/appendix.md](appendix/appendix.md): Go の SIMD の 2 つの隠れた性能上限(VZEROUPPER の遷移ペナルティ、register spill)、MaxSim、AVX-512 popcount、実行環境の調査(Apple Silicon、Rosetta、Docker、amd64 実機)

## 共有

- images/: 図(SVG と PNG)。教材と付録から参照
