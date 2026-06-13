# docs — ドキュメント索引

読者別に整理してある。各ファイルの役割は1つだけ。

## 参加者(ワークショップ受講者)向け
- **[workshop/workshop.html](workshop/workshop.html)** — 教材本体(これ一つ)。
  インタラクティブなルーフライン図 + 進め方(測る→AI→当たる天井→その天井を狙う手) +
  各 Stage の点と天井 + 天井の計測方法 + register spill の話 + 原典。
  点をクリックすると当たっている天井と Go コードが切り替わる。
  > 数値の正本はこの教材と、`make roofline` / `make roofline-ceiling`(実測・再現可能)。

## 開発・運営向け
- **[dev/OPTIMIZATION_LOG.md](dev/OPTIMIZATION_LOG.md)** — 実験の生ログ(罠と診断、Step 0〜6b、objdump、天井の実測)。
- **[dev/ENVIRONMENT_SURVEY.md](dev/ENVIRONMENT_SURVEY.md)** — 実行環境リファレンス(arm64 / Rosetta / Docker / amd64)。

## 共有
- **images/** — 図(SVG + PNG)。教材・記事から参照。

ルートの `README.md` = 入口、`PROPOSAL.md` = CFP プロポーザル。

---

### workshop.html の見せ方(GitHub だと注意)
GitHub は `.md` をその場でプレビューするが **`.html` はソース表示**になる。教材を見るには:
- **ローカルで開く**: `open docs/workshop/workshop.html`(自己完結の1ファイル。画像のみ `../images/` を参照)
- **当日**: 講師画面に投影してクリック操作
- **常時公開したい場合**: GitHub Pages を有効化(Settings → Pages → branch `main` / `/docs`)すると
  `https://<user>.github.io/gocon2026-simd-search/workshop/workshop.html` で配信できる
