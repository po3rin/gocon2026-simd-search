// Package vec は本ワークショップが Stage ごとに速くしていく距離カーネルを提供する。
// スカラ基準 → SIMD 化 → int8 量子化 → バイナリ量子化、の順で進む。
// すべての内積・距離関数は「a と b は同じ長さ」を事前条件とする(防御はしない)。
package vec

// DotNaive は Stage 0 の基準実装。1 要素ずつスカラで掛けて足す。
func DotNaive(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
