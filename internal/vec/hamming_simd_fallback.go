//go:build !(goexperiment.simd && amd64)

package vec

// HasVPOPCNT reports whether the AVX-512 popcount bonus path is usable.
func HasVPOPCNT() bool { return false }

// HammingSIMD falls back to the scalar implementation.
func HammingSIMD(a, b []uint64) int { return Hamming(a, b) }
