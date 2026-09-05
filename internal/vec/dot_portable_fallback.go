//go:build !goexperiment.simd

package vec

// DotPortable falls back to the scalar implementation when the simd
// package is not compiled in (GOEXPERIMENT=simd 未指定)。
func DotPortable(a, b []float32) float32 { return DotNaive(a, b) }

// PortableVectorBits returns 0 when the simd package is not compiled in.
func PortableVectorBits() int { return 0 }

// PortableEmulated returns false when the simd package is not compiled in.
func PortableEmulated() bool { return false }
