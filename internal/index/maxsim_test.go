package index

import (
	"math"
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
	if len(simd) != len(naive) {
		t.Fatalf("got %d results, want %d", len(simd), len(naive))
	}
	for i := range naive {
		if naive[i].ID != simd[i].ID {
			t.Errorf("rank %d: naive=%v simd=%v", i, naive[i], simd[i])
		}
	}
}

// MaxSim の式そのものを、独立に添字計算した参照実装と突き合わせる。
// Naive/SIMD の相互比較は共通の maxSim を通るため、式や DocToken の
// オフセット計算のバグは検出できない。ここでは mx.Data を直接添字で読む。
func TestMaxSimAgainstReference(t *testing.T) {
	r := rand.New(rand.NewPCG(35, 36))
	const (
		dim  = 4
		dTok = 3
		qTok = 2
		docs = 5
	)
	mx := NewMulti(dim, dTok)
	doc := make([][]float32, dTok)
	for i := 0; i < docs; i++ {
		for tk := range doc {
			v := make([]float32, dim)
			for j := range v {
				v[j] = float32(r.NormFloat64())
			}
			doc[tk] = v
		}
		mx.Add(doc)
	}
	q := make([][]float32, qTok)
	for tk := range q {
		v := make([]float32, dim)
		for j := range v {
			v[j] = float32(r.NormFloat64())
		}
		q[tk] = v
	}

	got := mx.SearchMaxSimNaive(q, docs)
	if len(got) != docs {
		t.Fatalf("got %d results, want %d", len(got), docs)
	}
	byID := make(map[int]float32, docs)
	for _, g := range got {
		byID[g.ID] = g.Score
	}
	for id := 0; id < docs; id++ {
		var want float64
		for _, qt := range q {
			best := math.Inf(-1)
			for dt := 0; dt < dTok; dt++ {
				off := (id*dTok + dt) * dim
				var dot float64
				for j := 0; j < dim; j++ {
					dot += float64(qt[j]) * float64(mx.Data[off+j])
				}
				if dot > best {
					best = dot
				}
			}
			want += best
		}
		if diff := float64(byID[id]) - want; diff > 1e-4 || diff < -1e-4 {
			t.Errorf("doc %d: got %f want %f", id, byID[id], want)
		}
	}
}

// 付録 2 節: MaxSim(late interaction)。文書トークン1ロードにつきクエリトークン
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
