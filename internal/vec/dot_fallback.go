//go:build !(goexperiment.simd && (amd64 || arm64))

package vec

// HasSIMD は SIMD 版がビルドに含まれていないので常に false。
// simd/archsimd は GOEXPERIMENT=simd のときだけ存在する(Go 1.27 では amd64 / arm64 / wasm)。
func HasSIMD() bool { return false }

// Dot は simd パッケージが無いビルド(GOEXPERIMENT 未指定、amd64 と arm64 以外)ではスカラ版に落ちる。
func Dot(a, b []float32) float32 { return DotNaive(a, b) }
