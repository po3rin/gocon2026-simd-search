package vec

import (
	"math/rand/v2"
	"testing"
)

func TestDotInt8MatchesNaive(t *testing.T) {
	r := rand.New(rand.NewPCG(21, 22))
	for _, n := range []int{0, 1, 15, 16, 17, 32, 384, 385} {
		a := make([]int8, n)
		b := make([]int8, n)
		for i := range a {
			a[i] = int8(r.IntN(255) - 127)
			b[i] = int8(r.IntN(255) - 127)
		}
		want := DotInt8Naive(a, b)
		got := DotInt8(a, b)
		if got != want { // 整数なので完全一致する
			t.Errorf("n=%d: DotInt8=%d DotInt8Naive=%d", n, got, want)
		}
	}
}

// 端の値と累積の上界を確認する。
//   - -128 同士: QuantizeInt8 は ±127 にクランプするので作らないが、
//     DotInt8 自体は -128 を含む入力でも正しい((-128)² の積和も int32 に収まる)
//   - 全要素 ±127 × dim=4096: アキュムレータの int32 ヘッドルームの確認
func TestDotInt8Extremes(t *testing.T) {
	a := make([]int8, 33) // 32 の倍数 + 端数
	b := make([]int8, 33)
	for i := range a {
		a[i] = -128
		b[i] = -128
	}
	if got, want := DotInt8(a, b), DotInt8Naive(a, b); got != want {
		t.Errorf("all -128: DotInt8=%d DotInt8Naive=%d", got, want)
	}

	big := make([]int8, 4096)
	for i := range big {
		big[i] = 127
	}
	want := int32(127) * 127 * 4096
	if got := DotInt8(big, big); got != want {
		t.Errorf("dim=4096 all 127: DotInt8=%d want %d", got, want)
	}
}

func TestQuantizeInt8RoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(23, 24))
	v := make([]float32, 384)
	for i := range v {
		v[i] = float32(r.NormFloat64())
	}
	q := make([]int8, 384)
	scale := QuantizeInt8(v, q)
	for i := range v {
		got := float32(q[i]) * scale
		// 丸めによる誤差は ±scale/2。ただし scale と inv(=127/maxAbs)を別々に
		// float32 で丸めているため厳密な scale/2 は保証されず、わずかな余裕を持たせる
		lim := scale * 0.5001
		if d := got - v[i]; d > lim || d < -lim {
			t.Errorf("i=%d: v=%f restored=%f (scale=%f)", i, v[i], got, scale)
		}
	}
}

// ゼロベクトルは scale=1 で全要素 0 に量子化される(godoc の仕様の確認)。
func TestQuantizeInt8Zero(t *testing.T) {
	v := make([]float32, 8)
	q := []int8{9, 9, 9, 9, 9, 9, 9, 9}
	if scale := QuantizeInt8(v, q); scale != 1 {
		t.Errorf("scale = %f, want 1", scale)
	}
	for i, x := range q {
		if x != 0 {
			t.Errorf("q[%d] = %d, want 0", i, x)
		}
	}
}

func BenchmarkDotInt8Naive(b *testing.B) {
	a8 := make([]int8, benchDim)
	b8 := make([]int8, benchDim)
	QuantizeInt8(benchA, a8)
	QuantizeInt8(benchB, b8)
	var sink int32
	for b.Loop() {
		sink = DotInt8Naive(a8, b8)
	}
	sinkI = int(sink)
}

func BenchmarkDotInt8SIMD(b *testing.B) {
	a8 := make([]int8, benchDim)
	b8 := make([]int8, benchDim)
	QuantizeInt8(benchA, a8)
	QuantizeInt8(benchB, b8)
	var sink int32
	for b.Loop() {
		sink = DotInt8(a8, b8)
	}
	sinkI = int(sink)
}
