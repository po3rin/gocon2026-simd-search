//go:build goexperiment.simd && amd64

package vec

import (
	"math"
	"testing"
	"unsafe"

	"simd/archsimd"
)

// Dot のコード生成比較ラボ。同じ計算を書き方だけ変えて測る。
// 実行: make remote-dotlab
//
// 比較軸:
//   - idx:    a[i:] のスライス式インデックス
//   - arr:    (*[8]float32) 配列ポインタ変換(境界チェックが消えるか?)
//   - unsafe: unsafe.Add によるポインタ演算(境界チェックゼロの理論値)
//   - 2/4:    アキュムレータ本数

func hsum8(v archsimd.Float32x8) float32 {
	var buf [8]float32
	v.StoreSlice(buf[:])
	return buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
}

func dotIdx2(a, b []float32) float32 {
	var acc0, acc1 archsimd.Float32x8
	for i := 0; i+16 <= len(a) && i+16 <= len(b); i += 16 {
		acc0 = archsimd.LoadFloat32x8Slice(a[i:]).MulAdd(archsimd.LoadFloat32x8Slice(b[i:]), acc0)
		acc1 = archsimd.LoadFloat32x8Slice(a[i+8:]).MulAdd(archsimd.LoadFloat32x8Slice(b[i+8:]), acc1)
	}
	return hsum8(acc0.Add(acc1))
}

func dotArr2(a, b []float32) float32 {
	var acc0, acc1 archsimd.Float32x8
	for i := 0; i+16 <= len(a) && i+16 <= len(b); i += 16 {
		acc0 = archsimd.LoadFloat32x8((*[8]float32)(a[i : i+8])).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(b[i:i+8])), acc0)
		acc1 = archsimd.LoadFloat32x8((*[8]float32)(a[i+8 : i+16])).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(b[i+8:i+16])), acc1)
	}
	return hsum8(acc0.Add(acc1))
}

func dotUnsafe2(a, b []float32) float32 {
	pa := unsafe.Pointer(unsafe.SliceData(a))
	pb := unsafe.Pointer(unsafe.SliceData(b))
	n := uintptr(min(len(a), len(b)) / 16 * 16 * 4)
	var acc0, acc1 archsimd.Float32x8
	for off := uintptr(0); off < n; off += 64 {
		acc0 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off))), acc0)
		acc1 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off+32))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off+32))), acc1)
	}
	return hsum8(acc0.Add(acc1))
}

func dotUnsafe4(a, b []float32) float32 {
	pa := unsafe.Pointer(unsafe.SliceData(a))
	pb := unsafe.Pointer(unsafe.SliceData(b))
	n := uintptr(min(len(a), len(b)) / 32 * 32 * 4)
	var acc0, acc1, acc2, acc3 archsimd.Float32x8
	for off := uintptr(0); off < n; off += 128 {
		acc0 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off))), acc0)
		acc1 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off+32))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off+32))), acc1)
		acc2 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off+64))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off+64))), acc2)
		acc3 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off+96))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off+96))), acc3)
	}
	return hsum8(acc0.Add(acc1).Add(acc2.Add(acc3)))
}

// unsafe2 と同一だが、ベクトル→スカラーの境界に VZEROUPPER を置く。
// これで速くなれば「レガシーSSE×dirty-ymm の遷移ペナルティ」が確定する。
func dotUnsafeVZ2(a, b []float32) float32 {
	pa := unsafe.Pointer(unsafe.SliceData(a))
	pb := unsafe.Pointer(unsafe.SliceData(b))
	n := uintptr(min(len(a), len(b)) / 16 * 16 * 4)
	var acc0, acc1 archsimd.Float32x8
	for off := uintptr(0); off < n; off += 64 {
		acc0 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off))), acc0)
		acc1 = archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pa, off+32))).MulAdd(archsimd.LoadFloat32x8((*[8]float32)(unsafe.Add(pb, off+32))), acc1)
	}
	var buf [8]float32
	acc0.Add(acc1).StoreSlice(buf[:])
	archsimd.ClearAVXUpperBits() // ← ここだけが unsafe2 との違い
	return buf[0] + buf[1] + buf[2] + buf[3] + buf[4] + buf[5] + buf[6] + buf[7]
}

func BenchmarkDotVariantUnsafeVZ2(b *testing.B) {
	for b.Loop() {
		sinkF = dotUnsafeVZ2(benchA, benchB)
	}
}

// 固定費(呼び出しごとのオーバーヘッド)切り分け用の大きい次元。
// dim=4096 でも遅ければループ本体、速くなれば固定費が犯人。
var benchBigFA, benchBigFB = func() ([]float32, []float32) {
	a := make([]float32, 4096)
	b := make([]float32, 4096)
	for i := range a {
		a[i] = float32(i%7) * 0.25
		b[i] = float32(i%5) * 0.5
	}
	return a, b
}()

func BenchmarkDotBigNaive(b *testing.B) {
	for b.Loop() {
		sinkF = DotNaive(benchBigFA, benchBigFB)
	}
}

func BenchmarkDotBigSIMD(b *testing.B) {
	for b.Loop() {
		sinkF = Dot(benchBigFA, benchBigFB)
	}
}

func BenchmarkDotBigUnsafe4(b *testing.B) {
	for b.Loop() {
		sinkF = dotUnsafe4(benchBigFA, benchBigFB)
	}
}

func TestDotVariants(t *testing.T) {
	if !HasSIMD() {
		t.Skip("no AVX2+FMA")
	}
	want := DotNaive(benchA, benchB)
	for name, fn := range map[string]func(a, b []float32) float32{
		"idx2": dotIdx2, "arr2": dotArr2, "unsafe2": dotUnsafe2, "unsafe4": dotUnsafe4, "unsafeVZ2": dotUnsafeVZ2,
	} {
		got := fn(benchA, benchB)
		if math.Abs(float64(got-want)) > 1e-3*(1+math.Abs(float64(want))) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}

func BenchmarkDotVariantIdx2(b *testing.B) {
	for b.Loop() {
		sinkF = dotIdx2(benchA, benchB)
	}
}

func BenchmarkDotVariantArr2(b *testing.B) {
	for b.Loop() {
		sinkF = dotArr2(benchA, benchB)
	}
}

func BenchmarkDotVariantUnsafe2(b *testing.B) {
	for b.Loop() {
		sinkF = dotUnsafe2(benchA, benchB)
	}
}

func BenchmarkDotVariantUnsafe4(b *testing.B) {
	for b.Loop() {
		sinkF = dotUnsafe4(benchA, benchB)
	}
}
