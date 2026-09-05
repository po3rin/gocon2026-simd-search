# GitHub Codespaces セットアップ手順（メンテナ向け / 当日運用）

本編ベンチ（Stage 0/1/2 + rerank）は **AVX2 + FMA だけ**で完結するので、Codespaces に
どの CPU（Intel/AMD・世代）が当たっても全ステージ再現できる。AVX-512 VPOPCNT は本編に
無いので CPU は問わない。実測の一例: 8-core は **AMD EPYC 7763 (Zen3)** が当たり、本編は
全て走った（AVX-512 は非搭載 = 付録は fallback。設計どおり）。

- 参加者向けの動線は [`docs/workshop/`](../workshop/) と [PROPOSAL.md](../../PROPOSAL.md) を参照。
- ここは **メンテナが CLI から Codespace を立てて `make bench` 等を回す**手順。実測値は
  [OPTIMIZATION_LOG.md](OPTIMIZATION_LOG.md) の Codespaces 節、環境比較は
  [ENVIRONMENT_SURVEY.md](ENVIRONMENT_SURVEY.md) に追記する。

---

## 0. 前提（一度だけ）

### 0-1. gh に codespace スコープを付与

```sh
gh auth refresh -h github.com -s codespace
# 表示される one-time code をブラウザ（github.com/login/device）に入力
gh auth status | grep -i scope   # 'codespace' が入っていることを確認
```

### 0-2. devcontainer に sshd feature が必要（反映済み）

`gh codespace ssh` はコンテナ内の sshd に繋ぐ。`mcr.microsoft.com/devcontainers/base:bookworm`
は sshd を起動しないので、`.devcontainer/devcontainer.json` に下記を入れてある（commit `be9d71e`）。
**これが無いと `failed to start SSH server / install sshd` で接続できない。**

```jsonc
"features": {
  "ghcr.io/devcontainers/features/go:1": { "version": "1.27" },
  "ghcr.io/devcontainers/features/sshd:1": { "version": "latest" }
}
```

### 0-3. ⚠️ ハマりどころ: `~/.ssh/codespaces.auto` が無いと publickey 拒否

sshd があっても `Permission denied (publickey,password)` で弾かれることがある。原因は
**gh が使う専用鍵 `~/.ssh/codespaces.auto` が生成されていない**（環境によっては gh が自動
生成に失敗する）。`gh codespace ssh --config` の `IdentityFile` がこの鍵を指しているのに
ファイルが無い、という状態。手で作れば直る（Codespace 側はいじらない。次回の接続で gh が
公開鍵を Codespace に登録する）:

```sh
ssh-keygen -t ed25519 -f ~/.ssh/codespaces.auto -N "" -q -C codespaces.auto
```

---

## 1. マシンサイズ（SKU 名に注意）

`gh codespace create -m <SKU>` の SKU 名はコア数と一致しない。**`standardLinux32gb` は
4-core（8-core ではない）**。8-core は `premiumLinux`。

| SKU | コア / RAM | 用途 |
|---|---|---|
| `basicLinux32gb` | 2 cores / 8 GB | 参加者の最小構成（無料枠を温存） |
| `standardLinux32gb` | **4 cores** / 16 GB | 参加者の標準 |
| `premiumLinux` | **8 cores** / 32 GB | メンテナの安定ベンチ用（本ログの実測はこれ） |
| `largePremiumLinux` | 16 cores / 64 GB | 通常不要 |

確認: `gh api '/repos/po3rin/gocon2026-simd-search/codespaces/machines'`

---

## 2. 立てる → 繋ぐ → 回す → 消す

```sh
REPO=po3rin/gocon2026-simd-search

# 立てる（8-core, idle 30分で自動停止）。参加者と同条件で測るなら -m standardLinux32gb（4-core）
# ※ gh 2.7x では -q フラグが無くなった（unknown shorthand flag: 'q'）。create は名前だけを stdout に出すので
#    そのまま受ける。--default-permissions を付けないと権限確認のプロンプトで止まることがある
CS=$(gh codespace create -R $REPO -b main -m premiumLinux --idle-timeout 30m --default-permissions 2>/dev/null | tail -1)

# Available になるまで待つ
until [ "$(gh codespace list --json name,state -q '.[]|select(.name=="'$CS'")|.state')" = "Available" ]; do sleep 8; done

# 繋ぐ（初回は鍵登録で数十秒かかることがある。0-3 の鍵が無いと publickey 拒否）
gh codespace ssh -c $CS -- 'nproc; lscpu | grep "Model name"'

# 本編フルを回す
gh codespace ssh -c $CS -- 'cd /workspaces/gocon2026-simd-search && \
  make isa-report && make bench && make recall && \
  make roofline-ceiling && make roofline-batch'

# ★必ず消す（課金停止。idle 自動停止はするが、止め忘れ防止に即 delete 推奨）
gh codespace delete -c $CS
```

リポジトリは Codespace 内の `/workspaces/gocon2026-simd-search` にチェックアウトされる。
`GOEXPERIMENT=simd` は devcontainer の `containerEnv` で設定済み。GOPATH は `/go`（`~/go` ではない）。

**測り方の注意（2026-09-05 の再計測で判明）:** 容器起動直後の 1 回目のベンチは遅めに出る
（DotSIMD が 56 ns のところ 71 ns）。数字を採るときは同じターゲットを 2 回走らせて 2 回目以降を使う。
手元の作業ツリーだけを試したいときは push せずに `gh codespace cp -e -c $CS <file> "remote:/workspaces/gocon2026-simd-search/<file>"` で 1 ファイルずつ送れる。

### make ターゲット早見

| ターゲット | 内容 |
|---|---|
| `make isa-report` | この CPU の archsimd 機能と各 Stage の active/fallback 一覧 |
| `make bench` | 本編フル: Naive / SIMD / Binary / BinaryRerank（AVX2+FMA だけ） |
| `make recall` | Recall@10（binary vs binary+rerank） |
| `make roofline-ceiling` | 天井の実測: PeakFLOP / PeakReadBW / PeakTriadBW |
| `make roofline-batch` | バッチ効果: B=1 SIMD vs B=32 naive/SIMD |
| `make bench-bonus` | （付録）AVX-512 VPOPCNT。Codespaces の AVX2 機では fallback |

---

## 3. 費用

- 計算リソースは **Codespace を起動した本人のアカウント**に課金される。
- メンテナのベンチ取り直しは 8-core で実質 4 コア時間程度（無料枠 120 コア時間/月の数%）。
  **「計測 → 即 delete」**を徹底すれば実質無料。
- 参加者は各自の無料枠で完結（40分・小さめマシンなら数%）。主催側は払わない。
  prebuild を有効化した場合のみ、その分が repo オーナー（po3rin）に課金される。

---

## 4. トラブルシュート早見

| 症状 | 原因 / 対処 |
|---|---|
| `unknown shorthand flag: 'q'` | gh 2.7x で `create -q` が廃止 → フラグを外す（create は名前だけを出力する） |
| `failed to start SSH server / install sshd` | devcontainer に sshd feature が無い → 0-2 を追加して作り直し |
| `Permission denied (publickey,password)` | `~/.ssh/codespaces.auto` が無い → 0-3 で生成 |
| `nproc` が想定より少ない | SKU 名の取り違え（`standardLinux32gb`=4-core）。8-core は `premiumLinux` |
| `make bench-bonus` が遅い/fallback | Codespaces の CPU は AVX-512 VPOPCNT 非搭載のことが多い（正常。付録は AVX-512 機向け） |
