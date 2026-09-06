package vec

import "math/bits"

// Words は dim ビットを入れるのに要る uint64 の語数を返す。
func Words(dim int) int { return (dim + 63) / 64 }

// Quantize は v の符号を out に詰める(v[i] > 0 のとき bit i が 1)。
// 1bit 量子化。float32 の 1 要素が 1bit になり、メモリは 1/32 になる。
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

// Hamming は a と b で値が違うビットの数を返す(Stage 4 のカーネル)。
// XOR と popcount だけで距離が出る。math/bits.OnesCount64 はスカラの POPCNT 命令になる。
// SIMD 版の popcount(VPOPCNTQ)は AVX512VPOPCNTDQ が要るので付録 3 節(make bench-bonus)で扱う。
func Hamming(a, b []uint64) int {
	var d int
	for i := range a {
		d += bits.OnesCount64(a[i] ^ b[i])
	}
	return d
}
