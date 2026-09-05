# 付録(深掘り資料)

本編([../workshop/workshop.md](../workshop/workshop.md))の 40 分には入らない、実装の裏側と調査の記録です。本編を読んだあとに、興味のあるものから読めます。

| ファイル | 内容 | こんなときに |
|---|---|---|
| [hidden-ceilings.md](hidden-ceilings.md) | Go の SIMD で見つかった 2 つの隠れた性能上限。VZEROUPPER の遷移ペナルティと register spill | 「演算ピークが理論値の 1/4 なのはなぜか」を知りたい |
| [environment-survey.md](environment-survey.md) | Apple Silicon、Rosetta、Docker、amd64 実機で SIMD がどう動くかの調査 | 手元の Mac や Docker で数字が出ない理由を知りたい |
| [optimization-log.md](optimization-log.md) | 実装を作る過程で踏んだつまずきと診断の記録(Step 0〜12)。本編の数字の出どころ | 「なぜこの実装になったか」「どう測ったか」をたどりたい |

数値は計測した機械ごとに違います。各ファイルの冒頭に計測環境を書いてあります。
