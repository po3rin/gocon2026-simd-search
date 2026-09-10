//go:build !goexperiment.simd

package vec

// DotPortable は simd パッケージが無いビルド(GOEXPERIMENT=simd 未指定)では
// スカラ実装にフォールバックする。
func DotPortable(a, b []float32) float32 { return DotNaive(a, b) }

// PortableVectorBits は simd パッケージが無いビルドでは 0 を返す。
func PortableVectorBits() int { return 0 }

// PortableEmulated は simd パッケージが無いビルドでは false を返す。
func PortableEmulated() bool { return false }
