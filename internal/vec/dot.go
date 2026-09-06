// Package vec はワークショップで Stage ごとに速くしていく距離カーネル。
// スカラの内積、SIMD の内積、int8 の内積、1bit のハミング距離。
package vec

// DotNaive は Stage 0 のベースライン。1 要素ずつスカラで掛けて足す。
func DotNaive(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
