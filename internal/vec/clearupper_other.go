//go:build goexperiment.simd && !amd64

package vec

// clearAVXUpperBits は amd64 以外では何もしない(AVX の遷移ペナルティは x86 固有)。
func clearAVXUpperBits() {}
