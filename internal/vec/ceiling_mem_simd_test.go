//go:build goexperiment.simd && amd64

package vec

import (
	"testing"

	"simd/archsimd"
)

// メモリ天井(AVX2 ストリーミング版)。検索カーネルと同じ 256bit ロードで
// 単コアが DRAM から引ける帯域を飽和させる。スカラー縮約版(_scalar)が
// 帯域を過小評価する問題への対処。詳細は ceiling_mem_test.go / OPTIMIZATION_LOG.md。

// BenchmarkPeakReadBW は読み取り専用の逐次ストリーム帯域を SIMD で測る。
// 256bit ロード×8本のアキュムレータで発行/レイテンシ律速を避け、純粋に
// DRAM 読み出しを飽和させる(検索の「順次読み」が到達できる帯域 = メモリ天井)。
func BenchmarkPeakReadBW(b *testing.B) {
	memSetup()
	if !hasSIMD {
		b.Skip("requires AVX2+FMA")
	}
	b.SetBytes(int64(memN) * 4)
	var sink float32
	iters := 0
	for b.Loop() {
		var a0, a1, a2, a3, a4, a5, a6, a7 archsimd.Float32x8
		x := memB
		for len(x) >= 64 {
			a0 = archsimd.LoadFloat32x8Slice(x).Add(a0)
			a1 = archsimd.LoadFloat32x8Slice(x[8:]).Add(a1)
			a2 = archsimd.LoadFloat32x8Slice(x[16:]).Add(a2)
			a3 = archsimd.LoadFloat32x8Slice(x[24:]).Add(a3)
			a4 = archsimd.LoadFloat32x8Slice(x[32:]).Add(a4)
			a5 = archsimd.LoadFloat32x8Slice(x[40:]).Add(a5)
			a6 = archsimd.LoadFloat32x8Slice(x[48:]).Add(a6)
			a7 = archsimd.LoadFloat32x8Slice(x[56:]).Add(a7)
			x = x[64:]
		}
		sum := a0.Add(a1).Add(a2.Add(a3)).Add(a4.Add(a5).Add(a6.Add(a7)))
		// ベクトル→スカラー境界(OPTIMIZATION_LOG.md の VZEROUPPER 税対策)
		archsimd.ClearAVXUpperBits()
		var buf [8]float32
		sum.StoreSlice(buf[:])
		sink += buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
		iters++
	}
	ceilSinkFloat = sink
	sec := b.Elapsed().Seconds()
	gb := float64(memN) * 4 * float64(iters) / sec / 1e9
	b.ReportMetric(gb, "read-GB/s")
}

// BenchmarkPeakTriadBW は STREAM Triad(a = b + s*c)を SIMD で測る。
// MulAdd(VFMADD)1発で c*s+b を計算して store。1要素あたり read b + read c +
// write a = 3 配列 ×4byte の論理転送(STREAM 慣習)。
func BenchmarkPeakTriadBW(b *testing.B) {
	memSetup()
	if !hasSIMD {
		b.Skip("requires AVX2+FMA")
	}
	s := fill8(3.0)
	b.SetBytes(int64(memN) * 3 * 4)
	iters := 0
	for b.Loop() {
		aa, bb, cc := memA, memB, memC
		for len(aa) >= 32 {
			archsimd.LoadFloat32x8Slice(cc).MulAdd(s, archsimd.LoadFloat32x8Slice(bb)).StoreSlice(aa)
			archsimd.LoadFloat32x8Slice(cc[8:]).MulAdd(s, archsimd.LoadFloat32x8Slice(bb[8:])).StoreSlice(aa[8:])
			archsimd.LoadFloat32x8Slice(cc[16:]).MulAdd(s, archsimd.LoadFloat32x8Slice(bb[16:])).StoreSlice(aa[16:])
			archsimd.LoadFloat32x8Slice(cc[24:]).MulAdd(s, archsimd.LoadFloat32x8Slice(bb[24:])).StoreSlice(aa[24:])
			aa = aa[32:]
			bb = bb[32:]
			cc = cc[32:]
		}
		iters++
	}
	archsimd.ClearAVXUpperBits()
	ceilSinkFloat = memA[memN-1]
	sec := b.Elapsed().Seconds()
	gb := float64(memN) * 3 * 4 * float64(iters) / sec / 1e9
	b.ReportMetric(gb, "triad-GB/s")
}
