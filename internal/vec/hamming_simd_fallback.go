//go:build !(goexperiment.simd && amd64)

package vec

// HasVPOPCNT は AVX-512 版がビルドに含まれていないので常に false。
func HasVPOPCNT() bool { return false }

// HammingSIMD はこのビルドではスカラ版に落ちる。
func HammingSIMD(a, b []uint64) int { return Hamming(a, b) }
