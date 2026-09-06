//go:build goexperiment.simd && amd64

package vec

import "simd/archsimd"

// hasSIMD はこの CPU で 256bit の FMA パスが使えるか。
// MulAdd は VFMADD213PS になるので、AVX2 に加えて FMA が要る。
var hasSIMD = archsimd.X86.AVX2() && archsimd.X86.FMA()

// HasSIMD は SIMD 版がビルドに含まれ、この CPU で使えるかを返す。
func HasSIMD() bool { return hasSIMD }

// Dot は 256bit の SIMD(float32 を 8 レーン)で内積を計算する(Stage 1)。
//
// 性能上のポイントは 2 つ。
//   - スライスは a[i:] で添字を付けず、a = a[16:] と前進させる。
//     添字式だと境界計算がループごとに走って支配的になる
//   - アキュムレータを 2 本にして、FMA の待ち時間を隠す
func Dot(a, b []float32) float32 {
	if !hasSIMD {
		return DotNaive(a, b)
	}
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	var acc0, acc1 archsimd.Float32x8 // ゼロ値は全要素 0
	for len(a) >= 16 {
		acc0 = archsimd.LoadFloat32x8(a).MulAdd(archsimd.LoadFloat32x8(b), acc0)
		acc1 = archsimd.LoadFloat32x8(a[8:]).MulAdd(archsimd.LoadFloat32x8(b[8:]), acc1)
		a = a[16:]
		b = b[16:]
	}
	if len(a) >= 8 {
		acc0 = archsimd.LoadFloat32x8(a).MulAdd(archsimd.LoadFloat32x8(b), acc0)
		a = a[8:]
		b = b[8:]
	}
	// 水平和: 2 本を 1 本に足してから 8 レーンをスカラへ
	var buf [8]float32
	acc0.Add(acc1).Store(buf[:])
	// ベクトルからスカラーへ戻る境界。Go は 1.27 でも VZEROUPPER を自動挿入しないため、
	// 標準 API の archsimd.ClearAVXUpperBits()(= VZEROUPPER)を自分で呼ぶ。
	// これを忘れると Intel 機では遷移ペナルティで呼び出しごとに数百サイクル失う
	// (docs/appendix/appendix.md の 1 節)
	archsimd.ClearAVXUpperBits()
	sum := buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	// 8 の倍数でない端数
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
