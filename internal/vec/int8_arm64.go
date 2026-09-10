//go:build goexperiment.simd && arm64

package vec

import "simd/archsimd"

// HasInt8SIMD reports whether the int8 SIMD path is compiled in and usable.
// arm64 では Neon が必須機能なので常に true。
func HasInt8SIMD() bool { return true }

// DotInt8 は int8 ベクトルの内積を Neon で計算する(Stage 2 の arm64 版)。
//
// amd64 版(int8_simd.go)は VPMADDWD(DotProductPairs)1発で int16 ペアの積和を
// int32 に落とせたが、Neon の archsimd(Go 1.27)にはその命令が無い。代わりに
//   - MulWidenLo(SMULL): int8 同士を掛けて int16 に広げる(下位8要素)
//   - HiToLo + MulWidenLo(SMULL2 相当): 上位8要素
//   - ExtendLo4ToInt32(SXTL): int16 → int32 に広げてから足し込む
//
// と 3 段で組む。int16 に積(最大 127×127=16129)は収まるが、積を int16 のまま
// 足し込むと 3 個目で溢れるので、必ず int32 へ広げてから蓄積する。
//
// 1 イテレーションで 16 要素。命令数は 2 SMULL + 4 SXTL + 4 ADD(+ ロード 2)で、
// スカラの 16 回 × (MUL+ADD) より少ない。アキュムレータは 1 レーンに 16 要素ごと
// ≦16129 を蓄積するので、dim ≦ 約 200 万まで int32 に収まる。
// a と b は同じ長さであること。
func DotInt8(a, b []int8) int32 {
	var acc0, acc1, acc2, acc3 archsimd.Int32x4
	for len(a) >= 16 {
		va := archsimd.LoadInt8x16(a)
		vb := archsimd.LoadInt8x16(b)
		lo := va.MulWidenLo(vb)                   // SMULL : a[0..7]*b[0..7] → int16x8
		hi := va.HiToLo().MulWidenLo(vb.HiToLo()) // SMULL2: a[8..15]*b[8..15]
		acc0 = acc0.Add(lo.ExtendLo4ToInt32())    // SXTL : int16 → int32(溢れ対策)
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
