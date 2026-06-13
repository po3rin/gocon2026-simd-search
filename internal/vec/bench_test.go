package vec

import (
	"math/rand/v2"
	"testing"
)

// カーネル単体ベンチ(データはL1キャッシュ内)。
// 全探索ベンチ(internal/index)との対比で「ALUの速さ」と
// 「メモリ帯域の壁」を切り分けるのが目的。
const benchDim = 384

var (
	benchA, benchB   []float32
	benchCA, benchCB []uint64
	benchBigA        []uint64
	benchBigB        []uint64
	sinkF            float32
	sinkI            int
)

func init() {
	r := rand.New(rand.NewPCG(9, 9))
	benchA = make([]float32, benchDim)
	benchB = make([]float32, benchDim)
	for i := range benchA {
		benchA[i] = float32(r.NormFloat64())
		benchB[i] = float32(r.NormFloat64())
	}
	benchCA = make([]uint64, Words(benchDim))
	benchCB = make([]uint64, Words(benchDim))
	Quantize(benchA, benchCA)
	Quantize(benchB, benchCB)
	// VPOPCNT の真価を測る用の大きめ配列(32KB×2、L1ギリギリ)
	benchBigA = make([]uint64, 4096)
	benchBigB = make([]uint64, 4096)
	for i := range benchBigA {
		benchBigA[i] = r.Uint64()
		benchBigB[i] = r.Uint64()
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

func BenchmarkHammingScalar(b *testing.B) {
	for b.Loop() {
		sinkI = Hamming(benchCA, benchCB)
	}
}

func BenchmarkHammingSIMD(b *testing.B) {
	for b.Loop() {
		sinkI = HammingSIMD(benchCA, benchCB)
	}
}

func BenchmarkHammingScalarBig(b *testing.B) {
	for b.Loop() {
		sinkI = Hamming(benchBigA, benchBigB)
	}
}

func BenchmarkHammingSIMDBig(b *testing.B) {
	for b.Loop() {
		sinkI = HammingSIMD(benchBigA, benchBigB)
	}
}
