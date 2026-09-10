//go:build goexperiment.simd && amd64

package vec

import "simd/archsimd"

// hasSIMD: この CPU で 256bit の FMA パスが使えるか。
// MulAdd は VFMADD213PS(FMA 拡張の命令)になる。FMA だけで足りるが、安全側に AVX2 も確認する。
var hasSIMD = archsimd.X86.AVX2() && archsimd.X86.FMA()

// HasSIMD は SIMD の高速パスがビルドに含まれ、実行環境で使えるかを返す。
func HasSIMD() bool { return hasSIMD }

// Dot は 256bit SIMD(float32 8 レーン)で内積を計算する。
// a と b は同じ長さであること(全カーネル共通の事前条件。Index は常に Dim で揃える)。
//
// 性能上のポイント2つ:
//   - スライスは誘導変数でインデックスせず a = a[16:] と前進させる。
//     インデックス式だと境界計算がループ毎に再実行されて支配的になる
//     (ループ内の a[8:] は定数オフセットなので境界計算は畳まれる)
//   - アキュムレータを2本にして FMA のレイテンシチェーンを分割する
func Dot(a, b []float32) float32 {
	if !hasSIMD {
		return DotNaive(a, b)
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
	// 水平加算: 2 本を 1 本に足してから、8 レーンをスカラーへ畳み込む
	var buf [8]float32
	acc0.Add(acc1).Store(buf[:])
	// ベクトルからスカラーへ戻る境界。Go は 1.27 でも VZEROUPPER を自動挿入しないため、
	// 標準 API の archsimd.ClearAVXUpperBits()(= VZEROUPPER)を自分で呼ぶ。
	// これを忘れると dirty ymm × レガシーSSE の遷移ペナルティで呼び出しごとに
	// 〜550サイクル失う(詳細: docs/appendix/appendix.md の 1 節)
	archsimd.ClearAVXUpperBits()
	sum := buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	// 端数(dim が 8 の倍数でない場合)
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
