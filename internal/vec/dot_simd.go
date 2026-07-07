//go:build goexperiment.simd && amd64

package vec

import "simd/archsimd"

// hasSIMD reports whether the CPU supports the 256-bit FMA path.
// MulAdd compiles to VFMADD213PS, which requires FMA in addition to AVX2.
var hasSIMD = archsimd.X86.AVX2() && archsimd.X86.FMA()

// HasSIMD reports whether the SIMD fast path is compiled in and usable.
func HasSIMD() bool { return hasSIMD }

// Dot computes the dot product using 256-bit SIMD (8 float32 lanes).
//
// Stage 1 の穴埋め対象。ワークショップ版ではループ本体が TODO になる。
//
// 性能上のポイント2つ:
//   - スライスは a[i:] でインデックスせず a = a[16:] と前進させる。
//     インデックス式だと境界計算がループ毎に再実行されて支配的になる
//   - アキュムレータを2本にして FMA のレイテンシチェーンを分割する
func Dot(a, b []float32) float32 {
	if !hasSIMD {
		return DotNaive(a, b)
	}
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	var acc0, acc1 archsimd.Float32x8 // ゼロ値は全要素 0
	for len(a) >= 16 {
		acc0 = archsimd.LoadFloat32x8Slice(a).MulAdd(archsimd.LoadFloat32x8Slice(b), acc0)
		acc1 = archsimd.LoadFloat32x8Slice(a[8:]).MulAdd(archsimd.LoadFloat32x8Slice(b[8:]), acc1)
		a = a[16:]
		b = b[16:]
	}
	if len(a) >= 8 {
		acc0 = archsimd.LoadFloat32x8Slice(a).MulAdd(archsimd.LoadFloat32x8Slice(b), acc0)
		a = a[8:]
		b = b[8:]
	}
	// 水平加算: 16レーンをスカラーに畳み込む
	var buf [8]float32
	acc0.Add(acc1).StoreSlice(buf[:])
	// ベクトル→スカラーの境界。Go 1.26 は VZEROUPPER を自動挿入しないため、
	// 標準 API の archsimd.ClearAVXUpperBits()(= VZEROUPPER)を自分で呼ぶ。
	// これを忘れると dirty ymm × レガシーSSE の遷移ペナルティで呼び出しごとに
	// 〜550サイクル失う(詳細: docs/dev/OPTIMIZATION_LOG.md)
	archsimd.ClearAVXUpperBits()
	sum := buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	// 端数(dim が 8 の倍数でない場合)
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
