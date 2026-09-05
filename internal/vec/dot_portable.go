//go:build goexperiment.simd

package vec

import "simd"

// DotPortable は Go 1.27 の ポータブル simd パッケージ(ベクトル長非依存)で
// 書いた内積。archsimd 版(dot_simd.go / dot_arm64.go)と違い、
//   - 型名にレーン数が無い(Float32s)。幅は実行時に CPU が決める:
//     AVX-512 機なら 512bit(16 レーン)、AVX2 機なら 256bit(8)、
//     arm64 Neon / wasm なら 128bit(4)
//   - amd64 / arm64 / wasm で同じソースが動く(ビルドタグ分岐が要らない)
//   - 命令が無いアーキでは純 Go でエミュレートされる(simd.Emulated() で判定)
//
// 幅が実行時に決まるので、ループ 1 周の要素数は n = acc.Len() から組み立てる。
// GODEBUG=simd=128 のように幅を狭めて実行できるので、「レジスタ幅を半分にしても
// 全探索は遅くならない(= メモリ律速)」を同じバイナリで確かめられる(make bench-portable)。
//
// 一方、Stage 3 の int8 積和(VPMADDWD / SMULL)や付録 B の popcount は
// ポータブル API には無い(アーキ間で共通に持てる演算だけが入っている)ので、
// 量子化カーネルは archsimd のままにしてある。
func DotPortable(a, b []float32) float32 {
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	var acc0, acc1 simd.Float32s // ゼロ値は全要素 0
	n := acc0.Len()              // このマシンのレーン数(4 / 8 / 16)
	for len(a) >= 2*n {
		acc0 = simd.LoadFloat32s(a).MulAdd(simd.LoadFloat32s(b), acc0)
		acc1 = simd.LoadFloat32s(a[n:]).MulAdd(simd.LoadFloat32s(b[n:]), acc1)
		a = a[2*n:]
		b = b[2*n:]
	}
	// 端数はマスク付きロード(足りないレーンはゼロ埋め)でベクトルのまま処理する
	for len(a) > 0 {
		va, k := simd.LoadFloat32sPart(a)
		vb, _ := simd.LoadFloat32sPart(b)
		acc0 = va.MulAdd(vb, acc0)
		a = a[k:]
		b = b[k:]
	}
	// 水平加算: ポータブル API には ReduceSum が無いので、ストアして足す
	var buf [16]float32 // 512bit = 16 レーンが上限
	acc0.Add(acc1).Store(buf[:n])
	clearAVXUpperBits() // amd64 のみ VZEROUPPER。他アーキでは no-op
	var sum float32
	for _, x := range buf[:n] {
		sum += x
	}
	return sum
}

// PortableVectorBits はポータブル simd がこの実行で使うベクトル幅(bit)を返す。
// GODEBUG=simd=<bits> で狭められる。
func PortableVectorBits() int { return simd.VectorBitSize() }

// PortableEmulated はポータブル simd がハードウェア命令ではなく純 Go の
// エミュレーションで動いているかを返す。
func PortableEmulated() bool { return simd.Emulated() }
