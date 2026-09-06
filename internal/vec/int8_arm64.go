//go:build goexperiment.simd && arm64

package vec

import "simd/archsimd"

// DotInt8 は int8 ベクトルの内積を Neon で計算する(Stage 3 の arm64 版)。
//
// amd64 版(int8_simd.go)は VPMADDWD(DotProductPairs)1 発で int16 のペアの積和を int32 に
// 落とせるが、Go 1.27 の archsimd の arm64 API にはそれにあたるメソッドが無い。代わりに 3 段で組む。
//   - MulWidenLo(SMULL): int8 同士を掛けて int16 に広げる(下位 8 要素)
//   - HiToLo と MulWidenLo(SMULL2 相当): 上位 8 要素
//   - ExtendLo4ToInt32(SXTL): int16 を int32 に広げてから足し込む
//
// 積(最大 127×127=16129)は int16 に収まるが、int16 のまま足し込むと 3 個目で溢れるので、
// 必ず int32 へ広げてから蓄積する。1 周で 16 要素。命令数は 2 SMULL + 4 SXTL + 4 ADD(+ ロード 2)で、
// スカラの 16 回 × (MUL+ADD) より少ない。
func DotInt8(a, b []int8) int32 {
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	var acc0, acc1, acc2, acc3 archsimd.Int32x4
	for len(a) >= 16 {
		va := archsimd.LoadInt8x16(a)
		vb := archsimd.LoadInt8x16(b)
		lo := va.MulWidenLo(vb)                   // SMULL : a[0..7]*b[0..7] を int16x8 に
		hi := va.HiToLo().MulWidenLo(vb.HiToLo()) // SMULL2: a[8..15]*b[8..15]
		acc0 = acc0.Add(lo.ExtendLo4ToInt32())    // SXTL : int16 を int32 に(溢れ対策)
		acc1 = acc1.Add(lo.HiToLo().ExtendLo4ToInt32())
		acc2 = acc2.Add(hi.ExtendLo4ToInt32())
		acc3 = acc3.Add(hi.HiToLo().ExtendLo4ToInt32())
		a = a[16:]
		b = b[16:]
	}
	s := acc0.Add(acc1).Add(acc2.Add(acc3)).ReduceSum() // ADDV: 4 レーンを 1 個に
	for i := range a {                                  // 16 の倍数でない端数
		s += int32(a[i]) * int32(b[i])
	}
	return s
}
