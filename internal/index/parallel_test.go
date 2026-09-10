package index

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

func TestSearchParallelMatchesSIMD(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	ix := New(64)
	v := make([]float32, 64)
	for i := 0; i < 1000; i++ {
		for j := range v {
			v[j] = float32(r.NormFloat64())
		}
		ix.Add(v)
	}
	q := make([]float32, 64)
	for j := range q {
		q[j] = float32(r.NormFloat64())
	}

	want := ix.SearchSIMD(q, 10)
	for _, workers := range []int{1, 2, 4, 7} {
		got := ix.SearchParallel(q, 10, workers)
		if len(got) != len(want) {
			t.Fatalf("workers=%d: got %d results, want %d", workers, len(got), len(want))
		}
		for i := range want {
			if got[i].ID != want[i].ID {
				t.Errorf("workers=%d rank %d: got %v want %v", workers, i, got[i], want[i])
			}
		}
	}
}

func TestSearchBatchParallelMatchesBatchSIMD(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	ix := New(64)
	v := make([]float32, 64)
	for i := 0; i < 1000; i++ {
		for j := range v {
			v[j] = float32(r.NormFloat64())
		}
		ix.Add(v)
	}
	qs := make([][]float32, 4)
	for b := range qs {
		q := make([]float32, 64)
		for j := range q {
			q[j] = float32(r.NormFloat64())
		}
		qs[b] = q
	}

	want := ix.SearchBatchSIMD(qs, 10)
	got := ix.SearchBatchParallel(qs, 10, 4)
	if len(got) != len(want) {
		t.Fatalf("got %d query results, want %d", len(got), len(want))
	}
	for b := range want {
		if len(got[b]) != len(want[b]) {
			t.Fatalf("query %d: got %d results, want %d", b, len(got[b]), len(want[b]))
		}
		for i := range want[b] {
			if got[b][i].ID != want[b][i].ID {
				t.Errorf("query %d rank %d: got %v want %v", b, i, got[b][i], want[b][i])
			}
		}
	}
}

// 寄り道: goroutine 並列はどの天井に効くか。
// メモリ律速の全探索(B=1)はコアが DRAM 帯域を取り合うのでサブリニア、
// 演算律速のバッチ(B=32)は物理コア数までほぼリニアに伸びる。
func BenchmarkSearchParallel(b *testing.B) {
	benchSetup()
	for _, w := range []int{1, 2, 4} {
		b.Run(fmt.Sprintf("workers=%d", w), func(b *testing.B) {
			b.SetBytes(benchN * benchDim * 4)
			for b.Loop() {
				benchIx.SearchParallel(benchQ, 10, w)
			}
		})
	}
}

func BenchmarkSearchBatchParallel(b *testing.B) {
	benchSetup()
	for _, w := range []int{1, 2, 4} {
		b.Run(fmt.Sprintf("workers=%d", w), func(b *testing.B) {
			iters := 0
			for b.Loop() {
				benchIx.SearchBatchParallel(benchQs, 10, w)
				iters++
			}
			sec := b.Elapsed().Seconds()
			b.ReportMetric(sec/float64(iters)/float64(benchBatch)*1e3, "ms/query")
		})
	}
}
