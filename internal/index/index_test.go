package index

import (
	"math/rand/v2"
	"testing"
)

func TestSearchAgreement(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
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

	naive := ix.SearchNaive(q, 10)
	simd := ix.SearchSIMD(q, 10)
	if len(naive) != 10 || len(simd) != 10 {
		t.Fatalf("got %d, %d results, want 10", len(naive), len(simd))
	}
	// FMA の丸め差で順位が入れ替わる可能性は理論上あるが、このデータでは
	// 完全一致することを確認する(入れ替わったらここで気付き、集合比較に緩める)
	for i := range naive {
		if naive[i].ID != simd[i].ID {
			t.Errorf("rank %d: naive=%v simd=%v", i, naive[i], simd[i])
		}
	}
	portable := ix.SearchPortable(q, 10)
	if len(portable) != len(naive) {
		t.Fatalf("portable: got %d results, want %d", len(portable), len(naive))
	}
	for i := range naive {
		if naive[i].ID != portable[i].ID {
			t.Errorf("rank %d: naive=%v portable=%v", i, naive[i], portable[i])
		}
	}
}

// バッチ検索は「B 本の単体検索と同じ答え」を返す。バッチ実装の添字の取り違えは
// Naive/SIMD の相互比較では検出できない(両方同じ形)ので、単体検索と突き合わせる。
func TestSearchBatchMatchesSingle(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 10))
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

	gotNaive := ix.SearchBatchNaive(qs, 10)
	gotSIMD := ix.SearchBatchSIMD(qs, 10)
	for b := range qs {
		wantN := ix.SearchNaive(qs[b], 10)
		wantS := ix.SearchSIMD(qs[b], 10)
		if len(gotNaive[b]) != len(wantN) || len(gotSIMD[b]) != len(wantS) {
			t.Fatalf("query %d: got %d/%d results, want %d/%d",
				b, len(gotNaive[b]), len(gotSIMD[b]), len(wantN), len(wantS))
		}
		for i := range wantN {
			if gotNaive[b][i].ID != wantN[i].ID {
				t.Errorf("naive query %d rank %d: got %v want %v", b, i, gotNaive[b][i], wantN[i])
			}
			if gotSIMD[b][i].ID != wantS[i].ID {
				t.Errorf("simd query %d rank %d: got %v want %v", b, i, gotSIMD[b][i], wantS[i])
			}
		}
	}
}

// BuildInt8 後に Add すると int8 表現は古くなるので捨てられる(作り直しが必要)。
func TestAddInvalidatesInt8(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	ix := New(8)
	v := make([]float32, 8)
	add := func() {
		for j := range v {
			v[j] = float32(r.NormFloat64())
		}
		ix.Add(v)
	}
	for i := 0; i < 10; i++ {
		add()
	}
	ix.BuildInt8()
	if ix.Codes8 == nil {
		t.Fatal("BuildInt8 should populate Codes8")
	}
	add()
	if ix.Codes8 != nil || ix.Scales != nil {
		t.Fatal("Add should invalidate the int8 representation")
	}
	ix.BuildInt8()
	if got := ix.SearchInt8(ix.Vec(10), 3); len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}
}

// TestRecall measures Recall@10 of the binary stage on clustered data.
// 合成データの作り方は helpers_test.go の clusteredIndex を参照。
func TestRecall(t *testing.T) {
	const (
		n   = 20_000
		dim = 384
		nq  = 50
		k   = 10
	)
	ix, sample := clusteredIndex(n, dim, 128, 7, 8)

	var recallBin, recallRerank float64
	for i := 0; i < nq; i++ {
		q := sample()
		exact := idSet(ix.SearchNaive(q, k))
		recallBin += overlap(exact, ix.SearchBinary(q, k))
		recallRerank += overlap(exact, ix.SearchBinaryRerank(q, k, 10))
	}
	recallBin /= nq * k
	recallRerank /= nq * k

	t.Logf("Recall@%d: binary=%.3f binary+rerank=%.3f", k, recallBin, recallRerank)
	if recallRerank < recallBin {
		t.Errorf("rerank should not hurt recall: binary=%.3f rerank=%.3f", recallBin, recallRerank)
	}
	if recallRerank < 0.8 {
		t.Errorf("rerank recall too low: %.3f", recallRerank)
	}
}
