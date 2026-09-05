# docs — ドキュメント索引

読者別に整理してある。各ファイルの役割は1つだけ。

## 参加者(ワークショップ受講者)向け
- **[workshop/SETUP.md](workshop/SETUP.md)** — まずここ。Codespaces ワンクリック起動・マシンサイズ・費用・フォールバック。
- **[workshop/workshop.md](workshop/workshop.md)** — 教材本体(これ一つ)。
  ルーフラインの進め方(測る、AI を出す、上限を見る、上限に効く手を打つ)、
  各 Stage の図、Go コードと計測コマンド、上限の計測方法、原典。
  本編は GitHub Codespaces(AMD EPYC 7763 で実測)前提。
  > 数値の正本はこの教材と、`make roofline` / `make roofline-ceiling`(実測・再現可能)。

## 付録(本編を読んだあとに)
- **[appendix/README.md](appendix/README.md)** — 付録の索引。
- **[appendix/hidden-ceilings.md](appendix/hidden-ceilings.md)** — Go の SIMD の 2 つの隠れた性能上限(VZEROUPPER の遷移ペナルティ、register spill)。
- **[appendix/environment-survey.md](appendix/environment-survey.md)** — 実行環境の調査(Apple Silicon、Rosetta、Docker、amd64 実機)。
- **[appendix/optimization-log.md](appendix/optimization-log.md)** — 最適化の記録(Step 0〜12。つまずきと診断、本編の数字の出どころ)。

## 共有
- **images/** — 図(SVG + PNG)。教材・記事から参照。

ルートの `README.md` が入口。

---

### workshop.md の見せ方
GitHub 上でそのままレンダリングされる(画像は `../images/` を参照)。当日は GitHub のページか、お好みの Markdown ビューアを講師画面に投影すればよい。
