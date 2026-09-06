//go:build !(goexperiment.simd && (amd64 || arm64))

package vec

// DotInt8 は simd パッケージが無いビルド(GOEXPERIMENT 未指定、amd64 と arm64 以外)ではスカラ版に落ちる。
func DotInt8(a, b []int8) int32 { return DotInt8Naive(a, b) }
