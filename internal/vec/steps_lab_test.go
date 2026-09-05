//go:build goexperiment.simd && amd64

package vec

import (
	"math"
	"math/bits"
	"testing"

	"simd/archsimd"
)

// docs/dev/OPTIMIZATION_LOG.md の各 Step の実装をコードとして保存したもの。
// `make remote-steps` で「高速化の階段」を Step 順に一気に再現できる。
//
//   Step 1: dotStep1     — 素朴な SIMD 化(インデックス式スライス + アキュムレータ1本)
//   Step 2: dotStep2     — スライス前進 + アキュムレータ2本(VZEROUPPER なし)
//   Step 5: vec.Dot      — Step 2 + VZEROUPPER(本実装)
//
// HammingSIMD も同様:
//   Step 2 以前: hammingStep1 — インデックス式 + アキュムレータ1本(VZEROUPPER なし)
//   本実装:      vec.HammingSIMD

// dotStep1 は最初に書いた素朴な SIMD 内積。
// 実測 219ns(naive 205ns より遅い!)。罠②の現物。
func dotStep1(a, b []float32) float32 {
	var acc archsimd.Float32x8
	i := 0
	for ; i+8 <= len(a); i += 8 {
		va := archsimd.LoadFloat32x8(a[i:])
		vb := archsimd.LoadFloat32x8(b[i:])
		acc = va.MulAdd(vb, acc)
	}
	var buf [8]float32
	acc.Store(buf[:])
	sum := buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	for ; i < len(a); i++ {
		sum += a[i] * b[i]
	}
	return sum
}

// dotStep2 はスライス前進 + アキュムレータ2本に書き換えた版(VZEROUPPER なし)。
// 実測 188ns。境界計算は減ったが「見えない税金」が残っている状態。
func dotStep2(a, b []float32) float32 {
	if len(b) < len(a) {
		a = a[:len(b)]
	}
	var acc0, acc1 archsimd.Float32x8
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
	var buf [8]float32
	acc0.Add(acc1).Store(buf[:])
	sum := buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// hammingStep1 は最初に書いた素朴な SIMD ハミング距離
// (インデックス式 + アキュムレータ1本、VZEROUPPER なし)。
// 検索ループに入れると 137ns/call に劣化していた版(SearchBinarySIMD 異常の現物)。
func hammingStep1(a, b []uint64) int {
	var acc archsimd.Uint64x4
	i := 0
	for ; i+4 <= len(a); i += 4 {
		va := archsimd.LoadUint64x4(a[i:])
		vb := archsimd.LoadUint64x4(b[i:])
		acc = acc.Add(va.Xor(vb).OnesCount())
	}
	var buf [4]uint64
	acc.Store(buf[:])
	d := int(buf[0] + buf[1] + buf[2] + buf[3])
	for ; i < len(a); i++ {
		d += bits.OnesCount64(a[i] ^ b[i])
	}
	return d
}

func TestStepsMatchNaive(t *testing.T) {
	if !HasSIMD() {
		t.Skip("no AVX2+FMA")
	}
	want := DotNaive(benchA, benchB)
	for name, fn := range map[string]func(a, b []float32) float32{
		"dotStep1": dotStep1, "dotStep2": dotStep2,
	} {
		got := fn(benchA, benchB)
		if math.Abs(float64(got-want)) > 1e-3*(1+math.Abs(float64(want))) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	if HasVPOPCNT() {
		if got, want := hammingStep1(benchCA, benchCB), Hamming(benchCA, benchCB); got != want {
			t.Errorf("hammingStep1 = %d, want %d", got, want)
		}
	}
}

// 「高速化の階段」を Step 順に再現するベンチ群。
// BenchmarkStepDot0〜3 を順に見ると OPTIMIZATION_LOG.md の数字をなぞれる。

func BenchmarkStepDot0Naive(b *testing.B) {
	for b.Loop() {
		sinkF = DotNaive(benchA, benchB)
	}
}

func BenchmarkStepDot1NaiveSIMD(b *testing.B) {
	for b.Loop() {
		sinkF = dotStep1(benchA, benchB)
	}
}

func BenchmarkStepDot2TwoAcc(b *testing.B) {
	for b.Loop() {
		sinkF = dotStep2(benchA, benchB)
	}
}

func BenchmarkStepDot3VZeroUpper(b *testing.B) {
	for b.Loop() {
		sinkF = Dot(benchA, benchB) // 本実装 = Step 2 + VZEROUPPER
	}
}

func BenchmarkStepHamming1NaiveSIMD(b *testing.B) {
	for b.Loop() {
		sinkI = hammingStep1(benchBigA, benchBigB)
	}
}

func BenchmarkStepHamming2Current(b *testing.B) {
	for b.Loop() {
		sinkI = HammingSIMD(benchBigA, benchBigB)
	}
}
