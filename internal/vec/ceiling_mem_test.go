package vec

import (
	"sync"
	"testing"
)

// ルーフラインの「メモリ天井」を実測するためのマイクロベンチ。
// 検索ワークロード(DB を順次ストリーム読み)に合わせ、シングルスレッドで測る。
//
//   - BenchmarkPeakReadBW  : 読み取り専用ストリーム帯域(検索に一番近い)
//   - BenchmarkPeakTriadBW : STREAM Triad a=b+s*c(参考・標準指標)
//
// 配列は LLC を確実に溢れさせる 256MB。GB/s を ReportMetric で出す。
// 詳細は docs/workshop/workshop.md。

const memN = 1 << 26 // 67,108,864 float32 = 256 MB(LLC 溢れ確実)

var (
	memOnce       sync.Once
	memA          []float32
	memB          []float32
	memC          []float32
	ceilSinkFloat float32
)

func memSetup() {
	memOnce.Do(func() {
		memA = make([]float32, memN)
		memB = make([]float32, memN)
		memC = make([]float32, memN)
		for i := range memB {
			memB[i] = float32(i&1023) * 0.5
			memC[i] = float32(i&511) * 0.25
		}
	})
}

// BenchmarkPeakReadBW は読み取り専用の逐次ストリーム帯域を測る。
// 8本の部分和でレイテンシ/発行律速を避け、純粋に DRAM 読み出しを飽和させる
// (検索カーネルと同じ「順次読み」の達成帯域 = メモリ天井の実測)。
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

// BenchmarkPeakTriadBW は STREAM Triad(a=b+s*c)の帯域を測る。
// 1要素あたり read b + read c + write a = 3 配列 ×4byte の論理転送(STREAM 慣習)。
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
