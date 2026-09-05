package vec

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestDotMatchesNaive(t *testing.T) {
	t.Logf("HasSIMD=%v HasVPOPCNT=%v", HasSIMD(), HasVPOPCNT())
	r := rand.New(rand.NewPCG(1, 2))
	// 8の倍数・端数あり・極小サイズを網羅
	for _, n := range []int{1, 7, 8, 9, 16, 100, 384, 768} {
		a := make([]float32, n)
		b := make([]float32, n)
		for i := range a {
			a[i] = float32(r.NormFloat64())
			b[i] = float32(r.NormFloat64())
		}
		want := DotNaive(a, b)
		got := Dot(a, b)
		// FMA は丸め順序が変わるので相対誤差で比較する
		if diff := math.Abs(float64(got - want)); diff > 1e-3*(1+math.Abs(float64(want))) {
			t.Errorf("n=%d: Dot=%v DotNaive=%v", n, got, want)
		}
	}
}

func TestQuantizeAndHamming(t *testing.T) {
	a := []float32{1, -1, 1, -1}
	b := []float32{1, 1, -1, -1}
	ca := make([]uint64, Words(len(a)))
	cb := make([]uint64, Words(len(b)))
	Quantize(a, ca)
	Quantize(b, cb)
	if ca[0] != 0b0101 {
		t.Errorf("Quantize(a) = %b, want 0101", ca[0])
	}
	if got := Hamming(ca, cb); got != 2 {
		t.Errorf("Hamming = %d, want 2", got)
	}
	if got := Hamming(ca, ca); got != 0 {
		t.Errorf("Hamming(self) = %d, want 0", got)
	}
}

func TestHammingSIMDMatchesScalar(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for _, words := range []int{1, 4, 6, 12, 16} {
		a := make([]uint64, words)
		b := make([]uint64, words)
		for i := range a {
			a[i] = r.Uint64()
			b[i] = r.Uint64()
		}
		if got, want := HammingSIMD(a, b), Hamming(a, b); got != want {
			t.Errorf("words=%d: HammingSIMD=%d Hamming=%d", words, got, want)
		}
	}
}

func TestDotPortableMatchesNaive(t *testing.T) {
	t.Logf("PortableVectorBits=%d PortableEmulated=%v", PortableVectorBits(), PortableEmulated())
	r := rand.New(rand.NewPCG(5, 6))
	for _, n := range []int{0, 1, 3, 4, 7, 8, 9, 15, 16, 17, 31, 32, 33, 100, 384, 768} {
		a := make([]float32, n)
		b := make([]float32, n)
		for i := range a {
			a[i] = float32(r.NormFloat64())
			b[i] = float32(r.NormFloat64())
		}
		want := DotNaive(a, b)
		got := DotPortable(a, b)
		if diff := math.Abs(float64(got - want)); diff > 1e-3*(1+math.Abs(float64(want))) {
			t.Errorf("n=%d: DotPortable=%v DotNaive=%v", n, got, want)
		}
	}
}
