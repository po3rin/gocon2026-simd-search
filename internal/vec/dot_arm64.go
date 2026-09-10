//go:build goexperiment.simd && arm64

package vec

import "simd/archsimd"

// hasSIMD: arm64 では Neon(128bit)が ARMv8-A の必須機能なので常に true。
// amd64 版(dot_simd.go)のような CPU 機能チェックは要らない。
var hasSIMD = true

// HasSIMD は SIMD の高速パスがビルドに含まれ、実行環境で使えるかを返す。
func HasSIMD() bool { return hasSIMD }

// Dot は Neon(Float32x4 = 128bit・4レーン)で内積を計算する(Stage 1 の arm64 版)。
//
// Go 1.27 で archsimd が arm64 に対応したので、Apple Silicon の Mac でも
// スカラ退避ではなく本物の SIMD が走る。構成は amd64 版と同じ:
//   - 1 イテレーションで 16 要素(4レーン × 4本)
//   - アキュムレータは 4 本。レーン幅が半分(4)なので、amd64 版の 2 本と
//     同じ「1周 16 要素」にするには 4 本要る。FMLA のレイテンシ隠しにも効く
//   - スライスは a = a[16:] と前進させる(境界計算をループ条件に吸収)
//   - VZEROUPPER 相当の後始末は arm64 には無い(AVX 特有の遷移ペナルティが存在しない)
//
// 本編(Codespaces / amd64)の数字とは別物なので、ここで出る倍率は
// 「自分の Mac の点」として読むこと(レジスタ幅 256→128、メモリ帯域も別)。
// a と b は同じ長さであること。
func Dot(a, b []float32) float32 {
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
	// 水平加算: 4本を1本に足してから 4 レーンをスカラーへ
	var buf [4]float32
	acc0.Add(acc1).Add(acc2.Add(acc3)).Store(buf[:])
	sum := buf[0] + buf[1] + buf[2] + buf[3]
	for i := range a { // 4 の倍数でない端数
		sum += a[i] * b[i]
	}
	return sum
}
