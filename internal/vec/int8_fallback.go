//go:build !(goexperiment.simd && (amd64 || arm64))

package vec

// HasInt8SIMD reports whether the int8 SIMD path is compiled in and usable.
func HasInt8SIMD() bool { return false }

// DotInt8 falls back to the scalar implementation on platforms without
// the simd package (GOEXPERIMENT 未指定のビルドや amd64/arm64 以外).
func DotInt8(a, b []int8) int32 { return DotInt8Naive(a, b) }
