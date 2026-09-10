package index

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
)

// N スイープ(docs 未掲載の実験): DB サイズを振って naive/SIMD を測る。
// DB がキャッシュ(L2/L3)に収まる間はメモリの壁が高い位置にあり SIMD が効くが、
// DRAM に溢れた瞬間に倍率が崩れる = 「SIMD が効く境界」はデータサイズの軸にもある。
var (
	sweepSizes = []int{1_000, 10_000, 100_000, 1_000_000}
	sweepOnce  sync.Once
	sweepIxs   map[int]*Index
	sweepQ     []float32
)

func sweepSetup() {
	sweepOnce.Do(func() {
		r := rand.New(rand.NewPCG(11, 12))
		sweepIxs = make(map[int]*Index, len(sweepSizes))
		v := make([]float32, benchDim)
		for _, n := range sweepSizes {
			ix := New(benchDim)
			for i := 0; i < n; i++ {
				for j := range v {
					v[j] = float32(r.NormFloat64())
				}
				ix.Add(v)
			}
			sweepIxs[n] = ix
		}
		sweepQ = make([]float32, benchDim)
		for j := range sweepQ {
			sweepQ[j] = float32(r.NormFloat64())
		}
	})
}

func BenchmarkSearchSweep(b *testing.B) {
	sweepSetup()
	for _, n := range sweepSizes {
		ix := sweepIxs[n]
		mb := float64(n) * benchDim * 4 / 1e6
		b.Run(fmt.Sprintf("naive/N=%d", n), func(b *testing.B) {
			for b.Loop() {
				ix.SearchNaive(sweepQ, 10)
			}
			b.ReportMetric(mb, "MB")
		})
		b.Run(fmt.Sprintf("simd/N=%d", n), func(b *testing.B) {
			for b.Loop() {
				ix.SearchSIMD(sweepQ, 10)
			}
			b.ReportMetric(mb, "MB")
		})
	}
}
