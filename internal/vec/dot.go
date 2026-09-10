// Package vec provides the distance kernels that the workshop optimizes
// stage by stage: scalar baseline → SIMD → int8 quantization → binary quantization.
// すべての内積・距離関数は「a と b は同じ長さ」を事前条件とする(防御はしない)。
package vec

// DotNaive is the Stage 0 baseline: one scalar multiply-add per element.
func DotNaive(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
