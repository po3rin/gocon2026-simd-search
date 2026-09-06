package vec

import "math/bits"

// Words returns the number of uint64 words needed to hold dim bits.
func Words(dim int) int { return (dim + 63) / 64 }

// Quantize packs the sign bits of v into out (bit i = 1 iff v[i] > 0).
// バイナリ量子化: float32 1要素 → 1bit。メモリは 1/32 になる。
func Quantize(v []float32, out []uint64) {
	for i := range out {
		out[i] = 0
	}
	for i, x := range v {
		if x > 0 {
			out[i/64] |= 1 << (i % 64)
		}
	}
}

// Hamming returns the number of differing bits between a and b.
//
// Stage 4 のカーネル。XOR + popcount だけで距離が出る。
// math/bits.OnesCount64 はスカラーの POPCNT 命令にコンパイルされる。
// SIMD 版 popcount(VPOPCNTQ)は AVX512VPOPCNTDQ が必要なので付録 3 節(make bench-bonus)で扱う。
func Hamming(a, b []uint64) int {
	var d int
	for i := range a {
		d += bits.OnesCount64(a[i] ^ b[i])
	}
	return d
}
