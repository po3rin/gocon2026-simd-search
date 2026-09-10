//go:build !(goexperiment.simd && (amd64 || arm64))

package vec

// HasInt8SIMD は int8 の SIMD パスがビルドに含まれ、実行環境で使えるかを返す。
func HasInt8SIMD() bool { return false }

// DotInt8 は simd パッケージが無いビルド(GOEXPERIMENT 未指定や amd64/arm64 以外)では
// スカラ実装にフォールバックする。
func DotInt8(a, b []int8) int32 { return DotInt8Naive(a, b) }
