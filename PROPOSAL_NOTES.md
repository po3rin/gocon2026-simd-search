# プロポーザル フォーム外メモ(提出しない・自分用)

[PROPOSAL.md](PROPOSAL.md) の補助メモ。Sessionize フォームには入力しない。

- リポジトリ: github.com/po3rin/gocon2026-simd-search(匿名審査のため PROPOSAL.md の A 欄には書かない)

## タイムテーブル(40分)

| 時間 | 内容 | 参加者がやること |
|---|---|---|
| 0–5分 | 導入: `GOEXPERIMENT=simd` とは / ルーフラインモデルとは「測る→AI→天井→手」 | 聞く(Codespacesは事前起動済み) |
| 5–9分 | Stage 0: naive全探索を測り、AI=0.5の点を図に打つ → 「どの天井にも未達」を確認 | `make roofline` で点を打つ |
| 9–17分 | Stage 1: 内積SIMD化を読む → 「カーネルは速いが全探索は伸びない」落差を図上の2点として読む(メモリ律速) | `make roofline` |
| 17–26分 | Stage 2: 「横に動け」①再利用。クエリのバッチ化で演算律速にし、**SIMDが exact のまま効く**のを確認 | `make roofline-batch`(B=1 vs B=32) 🎉 |
| 26–35分 | Stage 3: 「横に動け」②データ削減。量子化(近似)→ **fp32 SIMD rerank で精度回復** | `make roofline` / `make recall` 🎉 |
| 35–40分 | まとめ: 「SIMDはどこで効くか」二本柱 / VZEROUPPER・int8 の持ち帰り | — |

## 当日の演出メモ

- 各Stage後に `docs/workshop/workshop.md` の該当 Stage のルーフライン図を見せて全員で「いまここ」を共有する(点が縦に上り、Stage 2で横へ飛ぶ流れを示す)
- Stage 1完了直後に `go build -gcflags=-S` で `VFMADD231PS` が出ていることを30秒見せる(「あなたのGoコードがこのCPU命令になった」)
- Stage 1の「全探索1.6x止まり」で、図のメモリ斜線に点が張り付くのを見せてから「幅を倍にしても天井は動かない」を予言→確認の順で出す(ルーフラインの予言力の山場)
- 量子化のrecall劣化は「具体的な誤ヒット例」を1つ仕込んでスクリーンに出す(数字より笑いと納得)
- 早く終わった人向けの改造ネタ: アキュムレータ本数を変える、`Float32x16`(AVX-512)に差し替え、`Uint64x4.OnesCount`、int8量子化でAIを右へ
- AVX-512デモ用に AWS c7i / GCP c3 など Sapphire Rapids世代のVMを1台用意

## 実測値(AWS c7i / Xeon 8488C、Go 1.26.4、10万ベクトル×384次元)

| Stage | ルーフライン上の位置 | 実測 | 倍率 |
|---|---|---|---|
| 0: naive 全探索 | AI 0.5・どの天井にも未達 | 27.0 ms/query (2.85 GF / 5.7 GB/s) | 1.0x |
| 1: SIMD 内積(カーネル単体) | L1常駐・演算側で上げ代 | 188 → 41 ns | **4.6x** |
| 1: SIMD 全探索 | AI 0.5・メモリ斜線に張り付き(9.2 GB/s ≒ 帯域の84%) | 16.7 ms/query | 1.6x ← SIMD不発(メモリ律速) |
| 2: クエリのバッチ化(B=32・exact) | AI≈16・演算律速側へ → SIMD が効く | 4.49 ms/query | **4.3x**(scalar比) ← SIMDの山場 |
| 3: バイナリ量子化(byte 1/32・近似) | 横に動いてDRAM律速を脱出 | 0.62 ms/query | 43x(Recall@10 0.18) |
| 仕上げ: + float32 SIMD rerank(精度回復) | 精度軸(Recall@10 0.18→0.87) | 0.67 ms/query | **40x** |
| bonus: unsafe + VZEROUPPER 最適化 | sub-ceilingを掃除しメモリ天井へ | 23.5 ns(カーネル) | 8.7x |

※ **上の倍率はこの c7i 固有の実測例**。CPU・キャッシュ・帯域で変わるため、当日は各自のベンチで点を打って確かめる(=数値を約束しない設計)。Codespaces(本番環境)での再計測は応募後に実施。
※ 調査の副産物として「Go 1.26 simd は VZEROUPPER を自動挿入せず、SIMD関数の呼び出しごとに〜550cycleの隠れ税が発生しうる」という(おそらく)未報告の知見を得た(modern Intel = Sapphire Rapids での実測)。ルーフライン上では「メモリ天井に届く前に越えるべき隠れ sub-ceiling」として現れる。詳細は docs/dev/OPTIMIZATION_LOG.md。golang/go への issue 報告予定(関連: #77647)。
