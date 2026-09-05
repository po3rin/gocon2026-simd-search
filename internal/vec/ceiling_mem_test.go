package vec

import "sync"

// ルーフラインの「メモリ天井」を実測するためのマイクロベンチの共有部分。
// 検索ワークロード(DB を順次ストリーム読み)に合わせ、シングルスレッドで測る。
//
// 実体の Benchmark は2つに分かれている:
//   - ceiling_mem_simd_test.go   : AVX2 ストリーミング(検索カーネルと同じロード幅)。
//     単コアが DRAM から実際に引ける帯域を測れるので、これが天井の実測値。
//   - ceiling_mem_arm64_test.go  : 同じものの Neon(128bit)版。Apple Silicon はこちら。
//   - ceiling_mem_scalar_test.go : SIMD の無いビルド向けのスカラー fallback。
//
// 配列は L3 キャッシュを確実に溢れさせる 256MB。GB/s を ReportMetric で出す。
// 詳細は docs/workshop/workshop.md。
//
// ※ スカラー縮約(a += x[i])でこれを測ると、float 加算の発行/レイテンシで律速して
//    本来の DRAM 帯域より低く出る(特に単コア帯域の高い AMD で顕著)。SIMD 全探索が
//    達成する帯域すら下回ってしまい、ルーフライン上で点が屋根の上に来てしまう。
//    そのため天井ベンチも検索と同じ SIMD ロードで測る。詳細は OPTIMIZATION_LOG.md。

const memN = 1 << 26 // 67,108,864 float32 = 256 MB(L3 溢れ確実)。64/32 で割り切れる

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
