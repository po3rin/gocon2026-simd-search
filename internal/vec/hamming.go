package vec

import "math/bits"

// Words は dim bit を収めるのに必要な uint64 の個数を返す。
func Words(dim int) int { return (dim + 63) / 64 }

// Quantize は v の符号ビットを out に詰める(v[i] > 0 のとき bit i が 1)。
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

// Hamming は a と b で異なるビットの数を返す。
//
// Stage 3 の距離計算。XOR + popcount だけで距離が出る。
// math/bits.OnesCount64 はスカラーの POPCNT 命令にコンパイルされる。
// SIMD 版 popcount(VPOPCNTQ)は AVX512VPOPCNTDQ が必要なので付録 6 節(make bench-bonus)で扱う。
func Hamming(a, b []uint64) int {
	var d int
	for i := range a {
		d += bits.OnesCount64(a[i] ^ b[i])
	}
	return d
}
