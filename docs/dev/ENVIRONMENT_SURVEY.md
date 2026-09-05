# 実行環境調査 — Apple Silicon / Rosetta / Docker / amd64 実機

2026-06-11 実施(Go 1.26.4)。調査コマンド: `make isa-report`, `make test`, ベンチ 1 回実行。

> **2026-09-05 追記(Go 1.27.1):** `archsimd` が arm64(Neon・128bit)に対応したので、下表の「arm64 ネイティブ」列は **Stage 1 SIMD 内積 ✅(Neon 版 `dot_arm64.go`)、Stage 3 int8 ✅(`int8_arm64.go`)** に変わった。M3 Pro 実測は [OPTIMIZATION_LOG.md Step 12](OPTIMIZATION_LOG.md)。Rosetta の `FMA=false` は Go 1.27.1 + macOS 26 でも変わらず(再確認済み)。API 名は 1.27 で `LoadFloat32x8Slice`→`LoadFloat32x8`、`StoreSlice`→`Store` に改訂(下表は新名で記載)。

参照:
- [simd/archsimd (pkg.go.dev)](https://pkg.go.dev/simd/archsimd) — API ごとの `CPU Feature` と `archsimd.X86` ランタイムチェック
- [Rosetta translation environment (Apple)](https://developer.apple.com/documentation/apple-silicon/about-the-rosetta-translation-environment) — AVX/AVX2 は翻訳、AVX-512 は非対応

## 結論（先に）

| 環境 | 正しさのテスト | Stage 0/2 (スカラー) | Stage 1 SIMD 内積 | 付録 AVX-512 | 本編ベンチ再現 |
|---|---|---|---|---|---|
| **arm64 ネイティブ** (Apple M3 Pro) | ✅ `make test` | ✅ | ✅ Neon 128bit(Go 1.27〜。1.26 は ❌) | ❌ | △ 動くが本編(AVX2)とは別の点 |
| **Rosetta** (`GOARCH=amd64`) | ✅ | ✅ | ❌ **FMA=false** | ❌ AVX-512 非対応 | △ 量子化は再現、SIMD 内積は不可 |
| **Docker `linux/amd64`** (Apple Silicon 上) | ❌ ビルドクラッシュ / CPUID 全 false | △ バイナリ実行のみ | ❌ | ❌ | ❌ 使わない |
| **amd64 実機** (Codespaces / AWS c7i) | ✅ | ✅ | ✅ | ✅ | ✅ `make remote-bench` |

**推奨**: 本編(Stage 0/1/2 + rerank)で使う SIMD は **AVX2 + FMA だけ**なので、参加者・記事の数字は **GitHub Codespaces 一本**で全ステージ取れる(当たる CPU の Intel/AMD・世代を問わず再現)。AVX-512 VPOPCNT は本編フロー外の**付録**で、`infra/` の AWS c7i でだけ実機確認する。Apple Silicon の手元では **arm64 で `make test`**（正しさ）、**Rosetta で Stage 2 まで動作確認**が現実的。

## 調査方法

`cmd/isa-report` が [pkg.go.dev/simd/archsimd](https://pkg.go.dev/simd/archsimd) に沿って、本リポが使う API と `archsimd.X86.*()` の対応を一覧する。

```sh
make isa-report-amd64 GO=$(go env GOPATH)/bin/go1.27.1   # GOARCH=amd64(Rosetta)。素の make isa-report は arm64 版の一覧
```

本リポのガード（コードと一致）:

```go
// internal/vec/dot_simd.go
hasSIMD = archsimd.X86.AVX2() && archsimd.X86.FMA()

// internal/vec/hamming_simd.go
hasVPOPCNT = archsimd.X86.AVX512() && archsimd.X86.AVX512VPOPCNTDQ()
```

## 1. arm64 ネイティブ (Apple M3 Pro)

```
make test          → PASS (dot_fallback / hamming スカラーパス)
make isa-report    → 全 archsimd.X86 = false
```

- `simd/archsimd` は **amd64 専用**だが、ビルドタグでスカラーフォールバックに切り替わりテストは通る
- Stage 1 / 付録 の SIMD コードはコンパイルされない（`goexperiment.simd && amd64`）

ベンチ参考 (1 クエリ, 10万件):

```
BenchmarkSearchNaive  33.1 ms/op  (arm64 スカラー)
```

## 2. Rosetta (`GOARCH=amd64` on Apple Silicon)

### archsimd.X86 (isa-report 実測)

| Feature | 値 | pkg.go.dev 上の意味 |
|---|---|---|
| AVX | **true** | `ClearAVXUpperBits` / VZEROUPPER |
| AVX2 | **true** | `LoadFloat32x8`, `Uint64x4.Xor` 等 |
| FMA | **false** | `Float32x8.MulAdd` ← **Stage 1 のボトルネック** |
| AVX512 | false | Apple ドキュメント: 非対応 |
| AVX512VPOPCNTDQ | false | `Uint64x4.OnesCount` |
| **HasSIMD** | **false** | → `Dot` は `DotNaive` にフォールバック |
| **HasVPOPCNT** | false | → `HammingSIMD` は `Hamming` にフォールバック |

### API 単位

| Stage | API | 要求 feature | Rosetta |
|---|---|---|---|
| 0 | `bits.OnesCount64` | スカラー POPCNT | ✅ |
| 1 | `LoadFloat32x8` | AVX2 | ✅ |
| 1 | `Float32x8.MulAdd` | **FMA** | ❌ |
| 1 | `archsimd.ClearAVXUpperBits` | AVX | ✅ (到達前にガードで落ちる) |
| 2 | `Hamming` | スカラー POPCNT | ✅ |
| 付録 | `Uint64x4.OnesCount` | AVX512VPOPCNTDQ | ❌ |
| 仕上げ | `SearchBinaryRerank` | binary ✅ + Dot は Naive | △ |

### ベンチ参考 (Rosetta, 1 クエリ)

```
BenchmarkSearchNaive         24.1 ms/op
BenchmarkSearchBinary         0.70 ms/op   ← スカラー量子化、SIMD 不要
BenchmarkSearchBinaryRerank   0.81 ms/op   ← rerank も DotNaive
```

量子化 Stage は Rosetta でも **約 34 倍**のオーダー感は出る。SIMD 内積・AVX-512 は再現不可。

### Apple ドキュメントとの整合

> Rosetta translates all x86_64 instructions, **including AVX and AVX2**, but **doesn't support AVX512**.

- AVX2=true はドキュメントと一致
- **FMA が CPUID で false** になるのは Rosetta の制約（以前「AVX 全体が動かない」という記述は誤り → 修正済み）

## 3. Docker `linux/amd64` (Apple Silicon ホスト)

```
uname -m          → x86_64
/proc/cpuinfo     → ARM Features (asimd, …)  ← ホスト CPU をエミュ露出
go test / go run  → ビルド中に panic / SIGSEGV（QEMU エミュの不安定さ）
isa-report-linux  → 実行はできるが archsimd.X86 は **すべて false**
```

- `docker run --platform linux/amd64` は Apple Silicon 上では **QEMU 系エミュレーション**
- x86 の CPUID を正しくエミュレートしないため、`archsimd.X86` は信頼できない
- **ベンチ・SIMD 検証には不向き**。amd64 Linux 実機か Codespaces を使う

## 4. amd64 実機 (記事・教材の本番環境)

AWS c7i (Sapphire Rapids) 上の実測値（`OPTIMIZATION_LOG.md` / `make remote-bench`）:

| Feature | c7i |
|---|---|
| AVX2, FMA | ✅ |
| AVX512, AVX512VPOPCNTDQ | ✅ |
| HasSIMD, HasVPOPCNT | ✅ |

```
SearchNaive          28.6 ms
SearchBinary          0.68 ms  (42x)
SearchBinaryRerank    0.73 ms  (39x, Recall@10=0.87)
```

## 環境選びフロー

```
正しさだけ確認したい (Apple Silicon)
  → make test  (arm64, フォールバック)

量子化 Stage まで手元で触りたい
  → GOARCH=amd64 GOEXPERIMENT=simd go test ./...  (Rosetta)

本編(Stage 0/1/2 + rerank)を再現したい = SIMD 内積・量子化・記事の数字
  → Codespaces / devcontainer (amd64 ホスト)  ← AVX2+FMA だけなのでどの CPU でも再現
  → make bench

(付録) AVX-512 VPOPCNT を実機で確かめたい
  → make remote-bench / make bench-bonus  (AWS c7i = AVX-512 + VPOPCNTDQ)

CPU feature を確認したい
  → make isa-report  (Rosetta 上で GOARCH=amd64)
  → 解釈は pkg.go.dev の CPU Feature 表と照合
```

## 関連ファイル

- `cmd/isa-report/main.go` — 環境調査ツール
- `Makefile` — `isa-report` ターゲット
- `../workshop/workshop.md` — Docker / Rosetta コラム（AVX-512 非対応 / FMA 制約）
- `OPTIMIZATION_LOG.md` — c7i 上の最適化実測
