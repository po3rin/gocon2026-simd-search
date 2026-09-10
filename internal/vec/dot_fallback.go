//go:build !(goexperiment.simd && (amd64 || arm64))

package vec

// HasSIMD は SIMD の高速パスがビルドに含まれ、実行環境で使えるかを返す。
// simd/archsimd は GOEXPERIMENT=simd のときだけ存在する(Go 1.27: amd64 / arm64 / wasm)。
func HasSIMD() bool { return false }

// Dot は simd パッケージが無いビルド(GOEXPERIMENT 未指定や amd64/arm64 以外)では
// スカラ実装にフォールバックする。
func Dot(a, b []float32) float32 { return DotNaive(a, b) }
