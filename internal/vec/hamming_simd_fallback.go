//go:build !(goexperiment.simd && amd64)

package vec

// HasVPOPCNT は AVX-512 popcount の付録パスが使えるかを返す。
func HasVPOPCNT() bool { return false }

// HammingSIMD はスカラ実装にフォールバックする。
func HammingSIMD(a, b []uint64) int { return Hamming(a, b) }
