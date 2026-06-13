//go:build goexperiment.simd && amd64

package vec

// vzeroupper zeroes the upper 128 bits of all YMM registers.
//
// Go コンパイラ(1.26時点)はスカラー浮動小数点をレガシーSSEで出力し、
// VZEROUPPER も挿入しない。ymm の上位を汚したままレガシーSSE命令を実行すると
// 新しめの Intel CPU では高額な遷移ペナルティが発生するため、
// ベクトル処理→スカラー処理の境界でこれを呼ぶ。
func vzeroupper()
