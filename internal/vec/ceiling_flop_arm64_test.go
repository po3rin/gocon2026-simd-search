//go:build goexperiment.simd && arm64

package vec

import (
	"testing"

	"simd/archsimd"
)

// ルーフラインの「演算天井」を Neon(Float32x4 = 128bit)で実測する。
// amd64 版 BenchmarkPeakFLOP_AVX2(ceiling_flop_test.go)の arm64 版。
// Apple Silicon で `make roofline-ceiling` を叩くとこれが走る。
//
// アキュムレータ12本、漸化式 a = a*m + c は AVX2 版と同じ。Apple M 系は FMA
// パイプが 4 本 × レイテンシ ~4cyc なので 12 本あれば飽和する。
// レーン数が 4 なので flop の数え方は 12 本 × 4 レーン × 2。

func fill4(v float32) archsimd.Float32x4 {
	var b [4]float32
	for i := range b {
		b[i] = v
	}
	return archsimd.LoadFloat32x4(b[:])
}

// BenchmarkPeakFLOP_NEON は 128bit FMLA(Float32x4)のピーク GFLOP/s を測る。
func BenchmarkPeakFLOP_NEON(b *testing.B) {
	m := fill4(0.9999)
	c := fill4(1.0)
	a0, a1, a2, a3 := fill4(0.5), fill4(1.5), fill4(2.5), fill4(3.5)
	a4, a5, a6, a7 := fill4(4.5), fill4(5.5), fill4(6.5), fill4(7.5)
	a8, a9, a10, a11 := fill4(8.5), fill4(9.5), fill4(10.5), fill4(11.5)
	const inner = 1 << 12
	iters := 0
	for b.Loop() {
		for j := 0; j < inner; j++ {
			a0 = a0.MulAdd(m, c)
			a1 = a1.MulAdd(m, c)
			a2 = a2.MulAdd(m, c)
			a3 = a3.MulAdd(m, c)
			a4 = a4.MulAdd(m, c)
			a5 = a5.MulAdd(m, c)
			a6 = a6.MulAdd(m, c)
			a7 = a7.MulAdd(m, c)
			a8 = a8.MulAdd(m, c)
			a9 = a9.MulAdd(m, c)
			a10 = a10.MulAdd(m, c)
			a11 = a11.MulAdd(m, c)
		}
		iters++
	}
	sum := a0.Add(a1).Add(a2.Add(a3)).
		Add(a4.Add(a5).Add(a6.Add(a7))).
		Add(a8.Add(a9).Add(a10.Add(a11)))
	var buf [4]float32
	sum.Store(buf[:])
	ceilSinkFloat = buf[0] + buf[1] + buf[2] + buf[3]
	sec := b.Elapsed().Seconds()
	flop := float64(iters) * inner * 12 * 4 * 2 // 12 acc × 4 lane × 2 flop/FMA
	b.ReportMetric(flop/sec/1e9, "GFLOP/s")
}
