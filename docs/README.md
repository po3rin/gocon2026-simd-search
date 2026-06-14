# docs — ドキュメント索引

読者別に整理してある。各ファイルの役割は1つだけ。

## 参加者(ワークショップ受講者)向け
- **[workshop/workshop.md](workshop/workshop.md)** — 教材本体(これ一つ)。
  ルーフライン(進め方: 測る→AI→当たる天井→その天井を狙う手) +
  各 Stage の点と天井(静止画) + Go コードと計測コマンド + 天井の計測方法 + register spill の話 + 原典。
  > 数値の正本はこの教材と、`make roofline` / `make roofline-ceiling`(実測・再現可能)。

## 開発・運営向け
- **[dev/OPTIMIZATION_LOG.md](dev/OPTIMIZATION_LOG.md)** — 実験の生ログ(罠と診断、Step 0〜6b、objdump、天井の実測)。
- **[dev/ENVIRONMENT_SURVEY.md](dev/ENVIRONMENT_SURVEY.md)** — 実行環境リファレンス(arm64 / Rosetta / Docker / amd64)。

## 共有
- **images/** — 図(SVG + PNG)。教材・記事から参照。

ルートの `README.md` = 入口、`PROPOSAL.md` = CFP プロポーザル。

---

### workshop.md の見せ方
GitHub 上でそのままレンダリングされる(画像は `../images/` を参照)。当日は GitHub のページか、お好みの Markdown ビューアを講師画面に投影すればよい。
