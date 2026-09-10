package index

import "testing"

// TestRecallInt8 はクラスタ構造のある合成データで int8(Stage 2)の Recall@10 を測る。
// binary(0.18)と違い、大きさの情報が残るので単体で実用域に入る(実測 0.948)。
// 合成データは TestRecall と同じ clusteredIndex(helpers_test.go)を同じシードで使う。
func TestRecallInt8(t *testing.T) {
	const (
		n   = 20_000
		dim = 384
		nq  = 50
		k   = 10
	)
	ix, sample := clusteredIndex(n, dim, 128, 7, 8)
	ix.BuildInt8()

	var recall float64
	for i := 0; i < nq; i++ {
		q := sample()
		exact := idSet(ix.SearchNaive(q, k))
		recall += overlap(exact, ix.SearchInt8(q, k))
	}
	recall /= nq * k

	t.Logf("Recall@%d: int8=%.3f", k, recall)
	if recall < 0.8 {
		t.Errorf("int8 recall too low: %.3f", recall)
	}
}

// Stage 2: int8 量子化(1/4 サイズ)での全探索。
// 整数演算なので GFLOP/s とは呼ばず Gop/s と表記する(数え方は同じ 2 op/要素)。
func BenchmarkSearchInt8(b *testing.B) {
	benchSetup()
	benchIx.BuildInt8()
	// 共有インデックスに 39MB の int8 表現を持ち越すと後続ベンチのキャッシュ条件が
	// 変わるので、このベンチの終わりで捨てる
	b.Cleanup(func() { benchIx.Codes8, benchIx.Scales = nil, nil })
	b.SetBytes(int64(benchN) * benchDim) // int8: 1 byte/要素
	iters := 0
	for b.Loop() {
		benchIx.SearchInt8(benchQ, 10)
		iters++
	}
	sec := b.Elapsed().Seconds()
	ops := float64(benchN) * benchDim * 2 * float64(iters)
	b.ReportMetric(ops/sec/1e9, "Gop/s")
	b.ReportMetric(2.0, "AI(flop/byte)") // 2 op / 1 byte
	b.ReportMetric(float64(benchN)*benchDim/1e6, "MB/query")
}
