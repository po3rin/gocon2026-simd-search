//go:build goexperiment.simd && arm64

package vec

import "simd/archsimd"

// hasSIMD: arm64 では Neon(128bit)が ARMv8-A の必須機能なので常に true。
// amd64 版(dot_simd.go)のような CPU 機能チェックは要らない。
var hasSIMD = true

// HasSIMD は SIMD 版がビルドに含まれ、この CPU で使えるかを返す。
func HasSIMD() bool { return hasSIMD }

// Dot は Neon(Float32x4。128bit で 4 レーン)で内積を計算する(Stage 1 の arm64 版)。
//
// Go 1.27 で archsimd が arm64 に対応したので、Apple Silicon の Mac でも本物の SIMD が走る。
// 構成は amd64 版と同じ。
//   - 1 周で 16 要素(4 レーン × 4 本)
//   - アキュムレータは 4 本。レーン数が半分なので、amd64 版の 2 本と同じ「1 周 16 要素」にするには 4 本要る
//   - スライスは a = a[16:] と前進させる
//   - VZEROUPPER にあたる後始末は無い(AVX 特有の遷移ペナルティは arm64 に存在しない)
//
// 本編(Codespaces、amd64)の数字とは別物なので、ここで出る倍率は自分の Mac の値として読む。
func Dot(a, b []float32) float32 {
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	var acc0, acc1, acc2, acc3 archsimd.Float32x4 // ゼロ値は全要素 0
	for len(a) >= 16 {
		acc0 = archsimd.LoadFloat32x4(a).MulAdd(archsimd.LoadFloat32x4(b), acc0) // FMLA
		acc1 = archsimd.LoadFloat32x4(a[4:]).MulAdd(archsimd.LoadFloat32x4(b[4:]), acc1)
		acc2 = archsimd.LoadFloat32x4(a[8:]).MulAdd(archsimd.LoadFloat32x4(b[8:]), acc2)
		acc3 = archsimd.LoadFloat32x4(a[12:]).MulAdd(archsimd.LoadFloat32x4(b[12:]), acc3)
		a = a[16:]
		b = b[16:]
	}
	for len(a) >= 4 {
		acc0 = archsimd.LoadFloat32x4(a).MulAdd(archsimd.LoadFloat32x4(b), acc0)
		a = a[4:]
		b = b[4:]
	}
	// 水平和: 4 本を 1 本に足してから 4 レーンをスカラへ
	var buf [4]float32
	acc0.Add(acc1).Add(acc2.Add(acc3)).Store(buf[:])
	sum := buf[0] + buf[1] + buf[2] + buf[3]
	for i := range a { // 4 の倍数でない端数
		sum += a[i] * b[i]
	}
	return sum
}
