//go:build !(goexperiment.simd && (amd64 || arm64))

package vec

// HasSIMD reports whether the SIMD fast path is compiled in and usable.
// simd/archsimd は GOEXPERIMENT=simd のときだけ存在する(Go 1.27: amd64 / arm64 / wasm)。
func HasSIMD() bool { return false }

// Dot falls back to the scalar implementation on platforms without
// the simd package (e.g. GOEXPERIMENT 未指定のビルドや amd64/arm64 以外).
func Dot(a, b []float32) float32 { return DotNaive(a, b) }
