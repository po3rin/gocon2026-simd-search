# docs — ドキュメント索引

読者別に整理してある。各ファイルの役割は1つだけ。

## 参加者(ワークショップ受講者)向け
- **[workshop/SETUP.md](workshop/SETUP.md)** — まずここ。Codespaces ワンクリック起動・マシンサイズ・費用・フォールバック。
- **[workshop/workshop.md](workshop/workshop.md)** — 教材本体(これ一つ)。
  ルーフラインの進め方(測る、AI を出す、上限を見る、上限に効く手を打つ)、
  各 Stage の図、Go コードと計測コマンド、上限の計測方法、原典。
  本編は GitHub Codespaces(AMD EPYC 7763 で実測)前提。
  > 数値の正本はこの教材と、`make roofline` / `make roofline-ceiling`(実測・再現可能)。

## 開発・運営向け
- **[dev/OPTIMIZATION_LOG.md](dev/OPTIMIZATION_LOG.md)** — 実験の作業ログ(つまずきと診断、Step 0〜12、objdump、上限の実測、Codespaces 再計測)。
- **[dev/CODESPACES.md](dev/CODESPACES.md)** — CLI から Codespace を立ててベンチを回す手順(sshd feature / SKU 名 / 鍵のつまずき)。
- **[dev/HIDDEN_CEILINGS.md](dev/HIDDEN_CEILINGS.md)** — (深掘り)VZEROUPPER の遷移ペナルティ / register spill の調査(Intel c7i 実測。本編からは外した読み物)。
- **[dev/ENVIRONMENT_SURVEY.md](dev/ENVIRONMENT_SURVEY.md)** — 実行環境リファレンス(arm64 / Rosetta / Docker / amd64)。
- **[dev/VIZ_TOOLING_PLAN.md](dev/VIZ_TOOLING_PLAN.md)** — 可視化・プロファイルツール導入プラン(pprof / codegen 可視化 / 対話的ルーフライン)。

## 共有
- **images/** — 図(SVG + PNG)。教材・記事から参照。

ルートの `README.md` = 入口、`PROPOSAL.md` = CFP プロポーザル。

---

### workshop.md の見せ方
GitHub 上でそのままレンダリングされる(画像は `../images/` を参照)。当日は GitHub のページか、お好みの Markdown ビューアを講師画面に投影すればよい。
