package index

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/po3rin/gocon2026-simd-search/internal/vec"
)

// ワークショップ本番は実埋め込みデータを使うが、ベンチの規模感は同じ:
// 10万ベクトル × 384次元。
const (
	benchN     = 100_000
	benchDim   = 384
	benchBatch = 32 // バッチ検索のクエリ本数(DB再利用で AI ≈ 0.5×B)
)

var (
	benchOnce sync.Once
	benchIx   *Index
	benchQ    []float32
	benchQs   [][]float32
)

func benchSetup() {
	benchOnce.Do(func() {
		r := rand.New(rand.NewPCG(42, 0))
		benchIx = New(benchDim)
		v := make([]float32, benchDim)
		for i := 0; i < benchN; i++ {
			for j := range v {
				v[j] = float32(r.NormFloat64())
			}
			benchIx.Add(v)
		}
		benchQ = make([]float32, benchDim)
		for j := range benchQ {
			benchQ[j] = float32(r.NormFloat64())
		}
		benchQs = make([][]float32, benchBatch)
		for b := range benchQs {
			q := make([]float32, benchDim)
			for j := range q {
				q[j] = float32(r.NormFloat64())
			}
			benchQs[b] = q
		}
	})
}

// reportFloatRoofline は内積系 Stage の「ルーフライン上の点」を出力する。
// 内積は要素あたり mul+add = 2 flop、DB を fp32 で1回読むので 4 byte。
// → AI = 0.5 flop/byte(クエリは10万件で使い回すのでキャッシュ常駐、DRAM 転送に数えない)。
// 詳細は docs/workshop/workshop.md。
func reportFloatRoofline(b *testing.B, iters int) {
	sec := b.Elapsed().Seconds()
	flop := float64(benchN) * benchDim * 2 * float64(iters)
	bytes := float64(benchN) * benchDim * 4
	b.ReportMetric(flop/sec/1e9, "GFLOP/s")
	b.ReportMetric(2.0/4.0, "AI(flop/byte)")
	b.ReportMetric(bytes/1e6, "MB/query")
}

// reportBinaryRoofline は量子化 Stage の点を出力する。
// バイナリ表現では転送量が 1ベクトル 48 byte(1/32)。MB/query の激減が
// 「横に動いて DRAM 律速を脱出した」ことを示す(演算が popcount に変わるため
// flop ベースの AI は内積と直接比較しない)。
func reportBinaryRoofline(b *testing.B) {
	bytes := float64(benchN) * float64(vec.Words(benchDim)) * 8
	b.ReportMetric(bytes/1e6, "MB/query")
}

// Stage 0: スカラー全探索(ベースライン)。AI 0.5・どの天井にも未達。
func BenchmarkSearchNaive(b *testing.B) {
	benchSetup()
	b.SetBytes(benchN * benchDim * 4)
	iters := 0
	for b.Loop() {
		benchIx.SearchNaive(benchQ, 10)
		iters++
	}
	reportFloatRoofline(b, iters)
}

// Stage 1: SIMD 内積。AI 0.5・メモリ斜線に張り付く(縦に上った先の天井)。
func BenchmarkSearchSIMD(b *testing.B) {
	benchSetup()
	b.SetBytes(benchN * benchDim * 4)
	iters := 0
	for b.Loop() {
		benchIx.SearchSIMD(benchQ, 10)
		iters++
	}
	reportFloatRoofline(b, iters)
}

// ポータブル simd 版(simd.Float32s。workshop.md §02)。SearchSIMD と同じ点に乗る(Codespaces 実測で確認済み)。
func BenchmarkSearchPortable(b *testing.B) {
	benchSetup()
	b.SetBytes(benchN * benchDim * 4)
	iters := 0
	for b.Loop() {
		benchIx.SearchPortable(benchQ, 10)
		iters++
	}
	reportFloatRoofline(b, iters)
	b.ReportMetric(float64(vec.PortableVectorBits()), "vec-bits")
}

// Stage 3: バイナリ量子化 + スカラー popcount。右上に動いて DRAM 律速を脱出。
func BenchmarkSearchBinary(b *testing.B) {
	benchSetup()
	b.SetBytes(benchN * benchDim / 8)
	for b.Loop() {
		benchIx.SearchBinary(benchQ, 10)
	}
	reportBinaryRoofline(b)
}

// 付録 6 節(本編フロー外): バイナリ量子化 + AVX-512 VPOPCNT。量子化後はキャッシュ律速で
// 速くならない(≧ SearchBinary)ことの確認用。make bench-bonus。
func BenchmarkSearchBinarySIMD(b *testing.B) {
	benchSetup()
	b.SetBytes(benchN * benchDim / 8)
	for b.Loop() {
		benchIx.SearchBinarySIMD(benchQ, 10)
	}
	reportBinaryRoofline(b)
}

// Stage 4(仕上げ): バイナリ検索 + float32 rerank(精度軸。Recall@10 0.18→0.87)。
func BenchmarkSearchBinaryRerank(b *testing.B) {
	benchSetup()
	for b.Loop() {
		benchIx.SearchBinaryRerank(benchQ, 10, 10)
	}
}

// reportBatchRoofline: バッチ検索(B本同時)の点。DRAM転送は B=1 と同じ(d を再利用)
// なので AI ≈ 0.5×B。GFLOP/s と「クエリあたり ms」で SIMD の効きを見る。
func reportBatchRoofline(b *testing.B, iters int) {
	sec := b.Elapsed().Seconds()
	flop := float64(benchN) * benchDim * 2 * benchBatch * float64(iters)
	b.ReportMetric(flop/sec/1e9, "GFLOP/s")
	b.ReportMetric(float64(benchBatch)*0.5, "AI(flop/byte)")
	b.ReportMetric(sec/float64(iters)/float64(benchBatch)*1e3, "ms/query")
}

// Batch: B本のクエリで DB ロードを再利用 → 演算律速側へ → SIMD が exact 検索でも効くか?
func BenchmarkSearchBatchNaive(b *testing.B) {
	benchSetup()
	b.SetBytes(benchN * benchDim * 4)
	iters := 0
	for b.Loop() {
		benchIx.SearchBatchNaive(benchQs, 10)
		iters++
	}
	reportBatchRoofline(b, iters)
}

func BenchmarkSearchBatchSIMD(b *testing.B) {
	benchSetup()
	b.SetBytes(benchN * benchDim * 4)
	iters := 0
	for b.Loop() {
		benchIx.SearchBatchSIMD(benchQs, 10)
		iters++
	}
	reportBatchRoofline(b, iters)
}
