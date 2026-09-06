//go:build goexperiment.simd

package vec

import "simd"

// DotPortable は Go 1.27 のポータブル simd パッケージ(ベクトル長に依存しない API)で書いた内積。
// archsimd 版(dot_simd.go、dot_arm64.go)との違いは 3 つ。
//   - 型名にレーン数が無い(Float32s)。幅は実行時に CPU が決める
//     (AVX-512 機なら 16 レーン、AVX2 機なら 8、arm64 の Neon と wasm なら 4)
//   - amd64 / arm64 / wasm で同じソースが動き、ビルドタグの分岐が要らない
//   - 命令の無い環境では純 Go でエミュレートされる(simd.Emulated() で分かる)
//
// 幅が実行時に決まるので、ループ 1 周の要素数は n = acc.Len() から組み立てる。
// GODEBUG=simd=128 で幅を狭めて実行できるので、同じバイナリで「幅を半分にすると
// ルーフライン上の点がどこへ動くか」を確かめられる(make bench-portable。workshop.md Stage 1 コラム)。
//
// Stage 3 の int8 の積和や popcount はポータブル API に無い(アーキ間で共通に持てる演算だけが
// 入っている)ので、量子化カーネルは archsimd のままにしてある。
func DotPortable(a, b []float32) float32 {
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	// アキュムレータは 4 本。幅が 4 レーンに狭まっても FMA の待ち行列が長くならないように。
	var acc0, acc1, acc2, acc3 simd.Float32s // ゼロ値は全要素 0
	n := acc0.Len()                          // このマシンのレーン数(4 / 8 / 16)
	for len(a) >= 4*n {
		acc0 = simd.LoadFloat32s(a).MulAdd(simd.LoadFloat32s(b), acc0)
		acc1 = simd.LoadFloat32s(a[n:]).MulAdd(simd.LoadFloat32s(b[n:]), acc1)
		acc2 = simd.LoadFloat32s(a[2*n:]).MulAdd(simd.LoadFloat32s(b[2*n:]), acc2)
		acc3 = simd.LoadFloat32s(a[3*n:]).MulAdd(simd.LoadFloat32s(b[3*n:]), acc3)
		a = a[4*n:]
		b = b[4*n:]
	}
	for len(a) >= n {
		acc0 = simd.LoadFloat32s(a).MulAdd(simd.LoadFloat32s(b), acc0)
		a = a[n:]
		b = b[n:]
	}
	// 端数はマスク付きロード(足りないレーンはゼロ埋め)でベクトルのまま処理する
	for len(a) > 0 {
		va, k := simd.LoadFloat32sPart(a)
		vb, _ := simd.LoadFloat32sPart(b)
		acc0 = va.MulAdd(vb, acc0)
		a = a[k:]
		b = b[k:]
	}
	// 水平和: ポータブル API には ReduceSum が無いので、ストアして足す
	var buf [16]float32 // 512bit = 16 レーンが上限
	acc0.Add(acc1).Add(acc2.Add(acc3)).Store(buf[:n])
	clearAVXUpperBits() // amd64 だけ VZEROUPPER。他のアーキでは何もしない
	var sum float32
	for _, x := range buf[:n] {
		sum += x
	}
	return sum
}

// PortableVectorBits はポータブル simd がこの実行で使うベクトル幅(bit)を返す。GODEBUG=simd=<bits> で狭められる。
func PortableVectorBits() int { return simd.VectorBitSize() }

// PortableEmulated はポータブル simd がハードウェア命令ではなく純 Go のエミュレーションで動いているかを返す。
func PortableEmulated() bool { return simd.Emulated() }
