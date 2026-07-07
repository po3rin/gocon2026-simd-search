//go:build !(goexperiment.simd && amd64)

package vec

// DotInt8 falls back to the scalar implementation on platforms without
// the simd package (arm64 や GOEXPERIMENT 未指定のビルド).
func DotInt8(a, b []int8) int32 { return DotInt8Naive(a, b) }
