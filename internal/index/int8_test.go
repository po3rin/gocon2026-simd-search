package index

import (
	"math/rand/v2"
	"testing"
)

// TestRecallInt8 measures Recall@10 of the int8 stage on clustered data
// (Stage 3)。binary(0.18)と違い、大きさの情報が残るので単体で実用域に入るはず。
func TestRecallInt8(t *testing.T) {
	const (
		n        = 20_000
		dim      = 384
		nCenters = 128
		nq       = 50
		k        = 10
	)
	r := rand.New(rand.NewPCG(7, 8))

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
	ix.BuildInt8()

	var recall float64
	for i := 0; i < nq; i++ {
		q := sample()
		exact := idSet(ix.SearchNaive(q, k))
		recall += overlap(exact, ix.SearchInt8(q, k))
	}
	recall /= nq * k

	t.Logf("Recall@%d: int8=%.3f", k, recall)
	if recall < 0.8 {
		t.Errorf("int8 recall too low: %.3f", recall)
	}
}

// Stage 3: int8 量子化(1/4 サイズ)での全探索。
func BenchmarkSearchInt8(b *testing.B) {
	benchSetup()
	benchIx.BuildInt8()
	b.SetBytes(benchN * benchDim) // int8: 1 byte/要素
	for b.Loop() {
		benchIx.SearchInt8(benchQ, 10)
	}
	b.ReportMetric(float64(benchN)*benchDim/1e6, "MB/query")
}
