package vec

import (
	"math/rand/v2"
	"testing"
)

// カーネル単体のベンチ(データは L1 キャッシュ内)。
// 全探索のベンチ(internal/index)と見比べて、計算の速さとメモリ帯域の上限を切り分ける。
const benchDim = 384

var (
	benchA, benchB []float32
	sinkF          float32
	sinkI          int
)

func init() {
	r := rand.New(rand.NewPCG(9, 9))
	benchA = make([]float32, benchDim)
	benchB = make([]float32, benchDim)
	for i := range benchA {
		benchA[i] = float32(r.NormFloat64())
		benchB[i] = float32(r.NormFloat64())
	}
}

func BenchmarkDotNaive(b *testing.B) {
	for b.Loop() {
		sinkF = DotNaive(benchA, benchB)
	}
}

func BenchmarkDotSIMD(b *testing.B) {
	for b.Loop() {
		sinkF = Dot(benchA, benchB)
	}
}

// Stage 1 コラム: ポータブル simd パッケージ版。archsimd 版(DotSIMD)との差と、
// GODEBUG=simd=128 で幅を狭めたときの変化を見る(make bench-portable)。
func BenchmarkDotPortable(b *testing.B) {
	for b.Loop() {
		sinkF = DotPortable(benchA, benchB)
	}
}
