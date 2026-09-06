//go:build goexperiment.simd && amd64

package vec

import (
	"math/bits"

	"simd/archsimd"
)

// hasVPOPCNT はこの CPU に AVX-512 の VPOPCNTQ があるか。
var hasVPOPCNT = archsimd.X86.AVX512() && archsimd.X86.AVX512VPOPCNTDQ()

// HasVPOPCNT は AVX-512 の popcount(付録 3 節)がこの CPU で使えるかを返す。
func HasVPOPCNT() bool { return hasVPOPCNT }

// HammingSIMD は AVX-512 の VPOPCNTQ(4 つの uint64 を 1 命令で popcount)でハミング距離を計算する。
//
// 付録 3 節。量子化後はキャッシュ律速なので、popcount を SIMD 化しても速くならないことの
// 確認用(make bench-bonus)。AVX-512 が無い CPU ではスカラ版に落ちる。
func HammingSIMD(a, b []uint64) int {
	if !hasVPOPCNT {
		return Hamming(a, b)
	}
	if len(b) < len(a) {
		a = a[:len(b)]
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
	// 呼び出し元のスカラの float コード(topK の比較など)を遷移ペナルティから守る
	archsimd.ClearAVXUpperBits()
	d := int(buf[0] + buf[1] + buf[2] + buf[3])
	for i := range a {
		d += bits.OnesCount64(a[i] ^ b[i])
	}
	return d
}
