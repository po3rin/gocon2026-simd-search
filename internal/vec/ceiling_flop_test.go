//go:build goexperiment.simd && amd64

package vec

import (
	"testing"

	"simd/archsimd"
)

// ルーフラインの「演算天井」を実測する。FMA を完全にレジスタ上で回し、
// メモリも依存連鎖も挟まずに FMA スループットだけを飽和させる。
//
// アキュムレータ12本: Sapphire Rapids は FMA 2 ユニット × レイテンシ〜4cyc なので
// 8本以上 in-flight で飽和する。12本なら ymm レジスタ(16本)に収まりスピルしない。
// 漸化式 a = a*m + c (m=0.9999, c=1) は固定点 10000 に収束し、オーバーフロー/
// 非正規化数を踏まない。詳細は docs/workshop/workshop.md。

func fill8(v float32) archsimd.Float32x8 {
	var b [8]float32
	for i := range b {
		b[i] = v
	}
	return archsimd.LoadFloat32x8Slice(b[:])
}

// BenchmarkPeakFLOP_AVX2 は 256bit FMA(Float32x8)のピーク GFLOP/s を測る。
// ワークショップ Stage 1 が使う幅なので、これが「縦に上る天井」の実測値。
func BenchmarkPeakFLOP_AVX2(b *testing.B) {
	if !hasSIMD {
		b.Skip("requires AVX2+FMA")
	}
	m := fill8(0.9999)
	c := fill8(1.0)
	a0, a1, a2, a3 := fill8(0.5), fill8(1.5), fill8(2.5), fill8(3.5)
	a4, a5, a6, a7 := fill8(4.5), fill8(5.5), fill8(6.5), fill8(7.5)
	a8, a9, a10, a11 := fill8(8.5), fill8(9.5), fill8(10.5), fill8(11.5)
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
	vzeroupper()
	var buf [8]float32
	sum.StoreSlice(buf[:])
	ceilSinkFloat = buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	sec := b.Elapsed().Seconds()
	flop := float64(iters) * inner * 12 * 8 * 2 // 12 acc × 8 lane × 2 flop/FMA
	b.ReportMetric(flop/sec/1e9, "GFLOP/s")
}

// BenchmarkPeakFLOP_AVX2_4acc は同じ FMA 飽和をアキュムレータ4本で測る。
// 12本(_AVX2)との比較で「スピルはレジスタ圧のせいか(本数を減らせば直る)/
// archsimd 値が常にメモリ常駐の根本問題か」を切り分けるための実験。
func BenchmarkPeakFLOP_AVX2_4acc(b *testing.B) {
	if !hasSIMD {
		b.Skip("requires AVX2+FMA")
	}
	m := fill8(0.9999)
	c := fill8(1.0)
	a0, a1, a2, a3 := fill8(0.5), fill8(1.5), fill8(2.5), fill8(3.5)
	const inner = 1 << 12
	iters := 0
	for b.Loop() {
		for j := 0; j < inner; j++ {
			a0 = a0.MulAdd(m, c)
			a1 = a1.MulAdd(m, c)
			a2 = a2.MulAdd(m, c)
			a3 = a3.MulAdd(m, c)
		}
		iters++
	}
	sum := a0.Add(a1).Add(a2.Add(a3))
	vzeroupper()
	var buf [8]float32
	sum.StoreSlice(buf[:])
	ceilSinkFloat = buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	sec := b.Elapsed().Seconds()
	flop := float64(iters) * inner * 4 * 8 * 2
	b.ReportMetric(flop/sec/1e9, "GFLOP/s")
}
