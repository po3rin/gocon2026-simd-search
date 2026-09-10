//go:build goexperiment.simd && amd64

package vec

import (
	"math/bits"

	"simd/archsimd"
)

// hasVPOPCNT reports whether the CPU has AVX-512 VPOPCNTQ.
var hasVPOPCNT = archsimd.X86.AVX512() && archsimd.X86.AVX512VPOPCNTDQ()

// HasVPOPCNT reports whether the AVX-512 popcount bonus path is usable.
func HasVPOPCNT() bool { return hasVPOPCNT }

// HammingSIMD computes the Hamming distance using AVX-512 VPOPCNTQ
// (4 つの uint64 を 1 命令で popcount する)。
//
// 付録 6 節: 量子化後はキャッシュ律速のため、popcount を SIMD 化しても速くならない
// ことの確認用(make bench-bonus)。AVX-512 が無い CPU ではスカラー版にフォールバックする。
func HammingSIMD(a, b []uint64) int {
	if !hasVPOPCNT {
		return Hamming(a, b)
	}
	var acc0, acc1 archsimd.Uint64x4
	for len(a) >= 8 {
		acc0 = acc0.Add(archsimd.LoadUint64x4(a).Xor(archsimd.LoadUint64x4(b)).OnesCount())
		acc1 = acc1.Add(archsimd.LoadUint64x4(a[4:]).Xor(archsimd.LoadUint64x4(b[4:])).OnesCount())
		a = a[8:]
		b = b[8:]
	}
	if len(a) >= 4 {
		acc0 = acc0.Add(archsimd.LoadUint64x4(a).Xor(archsimd.LoadUint64x4(b)).OnesCount())
		a = a[4:]
		b = b[4:]
	}
	var buf [4]uint64
	acc0.Add(acc1).Store(buf[:])
	// 呼び出し元のスカラー float コード(topK の比較など)を遷移ペナルティから守る
	archsimd.ClearAVXUpperBits()
	d := int(buf[0] + buf[1] + buf[2] + buf[3])
	for i := range a {
		d += bits.OnesCount64(a[i] ^ b[i])
	}
	return d
}
