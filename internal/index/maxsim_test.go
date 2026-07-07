package index

import (
	"math/rand/v2"
	"sync"
	"testing"
)

func TestMaxSimAgreement(t *testing.T) {
	r := rand.New(rand.NewPCG(31, 32))
	mx := NewMulti(64, 4)
	doc := make([][]float32, 4)
	for i := 0; i < 500; i++ {
		for tk := range doc {
			v := make([]float32, 64)
			for j := range v {
				v[j] = float32(r.NormFloat64())
			}
			doc[tk] = v
		}
		mx.Add(doc)
	}
	q := make([][]float32, 8)
	for tk := range q {
		v := make([]float32, 64)
		for j := range v {
			v[j] = float32(r.NormFloat64())
		}
		q[tk] = v
	}

	naive := mx.SearchMaxSimNaive(q, 10)
	simd := mx.SearchMaxSimSIMD(q, 10)
	for i := range naive {
		if naive[i].ID != simd[i].ID {
			t.Errorf("rank %d: naive=%v simd=%v", i, naive[i], simd[i])
		}
	}
}

// 付録B: MaxSim(late interaction)。文書トークン1ロードにつきクエリトークン
// Tq 本と内積する構造がタスクに内在 = 最初から演算律速で SIMD が最初から効く。
const (
	maxsimDocs = 10_000
	maxsimDTok = 4  // 文書側トークン数
	maxsimQTok = 16 // クエリ側トークン数
)

var (
	maxsimOnce sync.Once
	maxsimIx   *MultiIndex
	maxsimQ    [][]float32
)

func maxsimSetup() {
	maxsimOnce.Do(func() {
		r := rand.New(rand.NewPCG(33, 34))
		maxsimIx = NewMulti(benchDim, maxsimDTok)
		doc := make([][]float32, maxsimDTok)
		for tk := range doc {
			doc[tk] = make([]float32, benchDim)
		}
		for i := 0; i < maxsimDocs; i++ {
			for tk := range doc {
				for j := range doc[tk] {
					doc[tk][j] = float32(r.NormFloat64())
				}
			}
			maxsimIx.Add(doc)
		}
		maxsimQ = make([][]float32, maxsimQTok)
		for tk := range maxsimQ {
			v := make([]float32, benchDim)
			for j := range v {
				v[j] = float32(r.NormFloat64())
			}
			maxsimQ[tk] = v
		}
	})
}

// AI = Tq×2×Dim×Tok flop ÷ Tok×Dim×4 byte = Tq/2 = 8 flop/byte(リッジの右)。
func reportMaxSimRoofline(b *testing.B, iters int) {
	sec := b.Elapsed().Seconds()
	flop := float64(maxsimDocs) * maxsimDTok * maxsimQTok * benchDim * 2 * float64(iters)
	b.ReportMetric(flop/sec/1e9, "GFLOP/s")
	b.ReportMetric(float64(maxsimQTok)/2, "AI(flop/byte)")
}

func BenchmarkSearchMaxSimNaive(b *testing.B) {
	maxsimSetup()
	iters := 0
	for b.Loop() {
		maxsimIx.SearchMaxSimNaive(maxsimQ, 10)
		iters++
	}
	reportMaxSimRoofline(b, iters)
}

func BenchmarkSearchMaxSimSIMD(b *testing.B) {
	maxsimSetup()
	iters := 0
	for b.Loop() {
		maxsimIx.SearchMaxSimSIMD(maxsimQ, 10)
		iters++
	}
	reportMaxSimRoofline(b, iters)
}
