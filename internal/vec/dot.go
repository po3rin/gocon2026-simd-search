// Package vec provides the distance kernels that the workshop optimizes
// stage by stage: scalar baseline → SIMD → binary quantization.
package vec

// DotNaive is the Stage 0 baseline: one scalar multiply-add per element.
func DotNaive(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
