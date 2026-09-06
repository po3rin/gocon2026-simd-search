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
		if d := got - v[i]; d > scale || d < -scale { // 量子化誤差は ±scale/2 以内
			t.Errorf("i=%d: v=%f restored=%f (scale=%f)", i, v[i], got, scale)
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
