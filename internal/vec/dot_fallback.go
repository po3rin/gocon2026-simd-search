//go:build !(goexperiment.simd && amd64)

package vec

// HasSIMD reports whether the SIMD fast path is compiled in and usable.
// simd/archsimd は GOEXPERIMENT=simd かつ amd64 のときだけ存在する。
func HasSIMD() bool { return false }

// Dot falls back to the scalar implementation on platforms without
// the simd package (e.g. arm64 や GOEXPERIMENT 未指定のビルド).
func Dot(a, b []float32) float32 { return DotNaive(a, b) }
