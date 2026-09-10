package index

import (
	"math"
	"math/rand/v2"
)

// clusteredIndex は Recall 計測用の合成データを作る。乱数そのままだと近傍に意味が
// 無いので、nCenters 個のクラスタ構造を持たせる。センターは成分 ~N(0,1) のまま使う
// (正規化すると成分がノイズに埋もれ、符号ビットが乱数化して binary 検索が成立しなくなる)。
// 戻り値の sample は、同じクラスタ構造からクエリ用のベクトルを引き続けるための関数。
// TestRecall(binary/rerank)と TestRecallInt8 が同じデータで測れるよう共有している。
func clusteredIndex(n, dim, nCenters int, seed1, seed2 uint64) (*Index, func() []float32) {
	r := rand.New(rand.NewPCG(seed1, seed2))
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
	return ix, sample
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
