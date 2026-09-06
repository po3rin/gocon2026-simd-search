//go:build goexperiment.simd && amd64

package vec

import "simd/archsimd"

// hasInt8SIMD はこの CPU で int8 の SIMD パスが使えるか。
// 使うのは VPMOVSXBW(拡張)、VPMADDWD(積和)、VPADDD で、すべて AVX2 で足りる(FMA は不要)。
var hasInt8SIMD = archsimd.X86.AVX2()

// DotInt8 は int8 ベクトルの内積を AVX2 で計算する(Stage 3)。
//
// 1 周で int8 を 16 個処理する。符号付きで int16x16 に広げ(VPMOVSXBW)、
// DotProductPairs(VPMADDWD)が「隣り合う 2 要素の積和」を int32x8 で返すので、それを足し込む。
// int16 同士の積は最大 127*127=16129 で、2 つ足しても int32 に余裕で収まる(飽和対策は要らない)。
func DotInt8(a, b []int8) int32 {
	if !hasInt8SIMD {
		return DotInt8Naive(a, b)
	}
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	var acc0, acc1 archsimd.Int32x8
	for len(a) >= 32 {
		acc0 = acc0.Add(archsimd.LoadInt8x16(a).ExtendToInt16().
			DotProductPairs(archsimd.LoadInt8x16(b).ExtendToInt16()))
		acc1 = acc1.Add(archsimd.LoadInt8x16(a[16:]).ExtendToInt16().
			DotProductPairs(archsimd.LoadInt8x16(b[16:]).ExtendToInt16()))
		a = a[32:]
		b = b[32:]
	}
	if len(a) >= 16 {
		acc0 = acc0.Add(archsimd.LoadInt8x16(a).ExtendToInt16().
			DotProductPairs(archsimd.LoadInt8x16(b).ExtendToInt16()))
		a = a[16:]
		b = b[16:]
	}
	var buf [8]int32
	acc0.Add(acc1).Store(buf[:])
	archsimd.ClearAVXUpperBits()
	s := buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	for i := range a { // 16 の倍数でない端数
		s += int32(a[i]) * int32(b[i])
	}
	return s
}
