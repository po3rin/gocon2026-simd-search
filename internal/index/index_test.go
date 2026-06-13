package index

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestSearchAgreement(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	ix := New(64)
	v := make([]float32, 64)
	for i := 0; i < 1000; i++ {
		for j := range v {
			v[j] = float32(r.NormFloat64())
		}
		ix.Add(v)
	}
	q := make([]float32, 64)
	for j := range q {
		q[j] = float32(r.NormFloat64())
	}

	naive := ix.SearchNaive(q, 10)
	simd := ix.SearchSIMD(q, 10)
	if len(naive) != 10 || len(simd) != 10 {
		t.Fatalf("got %d, %d results, want 10", len(naive), len(simd))
	}
	// SIMD は丸め差で順位が入れ替わりうるが、top-10 集合はほぼ一致するはず
	for i := range naive {
		if naive[i].ID != simd[i].ID {
			t.Errorf("rank %d: naive=%v simd=%v", i, naive[i], simd[i])
		}
	}
}

// TestRecall measures Recall@10 of the binary stage on clustered data.
// 乱数そのままだと近傍に意味がないので、クラスタ構造を持たせた合成データを使う。
func TestRecall(t *testing.T) {
	const (
		n        = 20_000
		dim      = 384
		nCenters = 128
		nq       = 50
		k        = 10
	)
	r := rand.New(rand.NewPCG(7, 8))

	// センターは成分 ~N(0,1) のまま使う(正規化すると成分がノイズに埋もれ、
	// 符号ビットが乱数化して binary 検索が成立しなくなる)
	centers := make([][]float32, nCenters)
	for i := range centers {
		c := make([]float32, dim)
		for j := range c {
			c[j] = float32(r.NormFloat64())
		}
		centers[i] = c
	}
	sample := func() []float32 {
		c := centers[r.IntN(nCenters)]
		v := make([]float32, dim)
		for j := range v {
			v[j] = c[j] + 0.4*float32(r.NormFloat64())
		}
		normalize(v)
		return v
	}

	ix := New(dim)
	for i := 0; i < n; i++ {
		ix.Add(sample())
	}

	var recallBin, recallRerank float64
	for i := 0; i < nq; i++ {
		q := sample()
		exact := idSet(ix.SearchNaive(q, k))
		recallBin += overlap(exact, ix.SearchBinary(q, k))
		recallRerank += overlap(exact, ix.SearchBinaryRerank(q, k, 10))
	}
	recallBin /= nq * k
	recallRerank /= nq * k

	t.Logf("Recall@%d: binary=%.3f binary+rerank=%.3f", k, recallBin, recallRerank)
	if recallRerank < recallBin {
		t.Errorf("rerank should not hurt recall: binary=%.3f rerank=%.3f", recallBin, recallRerank)
	}
	if recallRerank < 0.8 {
		t.Errorf("rerank recall too low: %.3f", recallRerank)
	}
}

func normalize(v []float32) {
	var ss float64
	for _, x := range v {
		ss += float64(x) * float64(x)
	}
	inv := float32(1 / math.Sqrt(ss))
	for i := range v {
		v[i] *= inv
	}
}

func idSet(rs []Result) map[int]bool {
	m := make(map[int]bool, len(rs))
	for _, r := range rs {
		m[r.ID] = true
	}
	return m
}

func overlap(exact map[int]bool, rs []Result) float64 {
	var hit float64
	for _, r := range rs {
		if exact[r.ID] {
			hit++
		}
	}
	return hit
}
