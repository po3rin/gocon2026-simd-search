//go:build !(goexperiment.simd && amd64)

package vec

import "testing"

// メモリ天井(スカラー fallback)。amd64/SIMD が無い環境(arm64 等)向け。
// 本番の天井計測は ceiling_mem_simd_test.go の SIMD 版で行う(こちらは縮約の
// 発行/レイテンシで律速し帯域を過小評価しうる)。詳細は ceiling_mem_test.go。

// BenchmarkPeakReadBW は読み取り専用の逐次ストリーム帯域を測る(スカラー8本)。
func BenchmarkPeakReadBW(b *testing.B) {
	memSetup()
	b.SetBytes(int64(memN) * 4)
	var s0, s1, s2, s3, s4, s5, s6, s7 float32
	iters := 0
	for b.Loop() {
		var a0, a1, a2, a3, a4, a5, a6, a7 float32
		for i := 0; i < memN; i += 8 {
			a0 += memB[i]
			a1 += memB[i+1]
			a2 += memB[i+2]
			a3 += memB[i+3]
			a4 += memB[i+4]
			a5 += memB[i+5]
			a6 += memB[i+6]
			a7 += memB[i+7]
		}
		s0, s1, s2, s3 = s0+a0, s1+a1, s2+a2, s3+a3
		s4, s5, s6, s7 = s4+a4, s5+a5, s6+a6, s7+a7
		iters++
	}
	ceilSinkFloat = s0 + s1 + s2 + s3 + s4 + s5 + s6 + s7
	sec := b.Elapsed().Seconds()
	gb := float64(memN) * 4 * float64(iters) / sec / 1e9
	b.ReportMetric(gb, "read-GB/s")
}

// BenchmarkPeakTriadBW は STREAM Triad(a=b+s*c)の帯域を測る(スカラー)。
func BenchmarkPeakTriadBW(b *testing.B) {
	memSetup()
	const scalar = 3.0
	b.SetBytes(int64(memN) * 3 * 4)
	iters := 0
	for b.Loop() {
		for i := 0; i < memN; i++ {
			memA[i] = memB[i] + scalar*memC[i]
		}
		iters++
	}
	ceilSinkFloat = memA[memN-1]
	sec := b.Elapsed().Seconds()
	gb := float64(memN) * 3 * 4 * float64(iters) / sec / 1e9
	b.ReportMetric(gb, "triad-GB/s")
}
