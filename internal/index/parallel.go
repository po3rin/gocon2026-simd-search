package index

import (
	"sync"

	"github.com/po3rin/gocon2026-simd-search/internal/vec"
)

// SearchParallel は SearchSIMD(SIMD 内積の全探索)を workers 本の goroutine で
// 並列化する。DB を連続チャンクに分割し、各 worker が自分のチャンクだけを
// 走査して worker ローカルの topK を作り、最後にマージする。
//
// 「寄り道」の実験対象: 全探索はメモリ律速なので、コアを増やしても
// DRAM 帯域を取り合うだけで理想の workers 倍にはならない(サブリニア)。
// docs/workshop/workshop.md の寄り道節を参照。
func (ix *Index) SearchParallel(q []float32, k, workers int) []Result {
	if workers <= 1 {
		return ix.SearchSIMD(q, k)
	}
	chunk := (ix.N + workers - 1) / workers
	tops := make([]*topK, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		lo := w * chunk
		hi := min(lo+chunk, ix.N)
		if lo >= hi {
			continue
		}
		tops[w] = newTopK(k)
		wg.Add(1)
		go func(t *topK, lo, hi int) {
			defer wg.Done()
			for id := lo; id < hi; id++ {
				t.push(id, vec.Dot(q, ix.Vec(id)))
			}
		}(tops[w], lo, hi)
	}
	wg.Wait()
	return mergeTopK(k, tops)
}

// SearchBatchParallel は SearchBatchSIMD(B 本のクエリを1パスで処理)を
// workers 本の goroutine で並列化する。分割の仕方は SearchParallel と同じ
// DB チャンク分割(worker ごとの AI は 0.5×B のまま変わらない)。
//
// バッチは演算律速なので、こちらは物理コア数までほぼ比例して速くなる。
func (ix *Index) SearchBatchParallel(qs [][]float32, k, workers int) [][]Result {
	if workers <= 1 {
		return ix.SearchBatchSIMD(qs, k)
	}
	chunk := (ix.N + workers - 1) / workers
	// tops[w][b]: worker w のクエリ b 用 topK
	tops := make([][]*topK, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		lo := w * chunk
		hi := min(lo+chunk, ix.N)
		if lo >= hi {
			continue
		}
		tops[w] = make([]*topK, len(qs))
		for b := range tops[w] {
			tops[w][b] = newTopK(k)
		}
		wg.Add(1)
		go func(ts []*topK, lo, hi int) {
			defer wg.Done()
			for id := lo; id < hi; id++ {
				d := ix.Vec(id)
				for b := range qs {
					ts[b].push(id, vec.Dot(qs[b], d))
				}
			}
		}(tops[w], lo, hi)
	}
	wg.Wait()
	out := make([][]Result, len(qs))
	for b := range out {
		per := make([]*topK, 0, workers)
		for w := range tops {
			if tops[w] != nil {
				per = append(per, tops[w][b])
			}
		}
		out[b] = mergeTopK(k, per)
	}
	return out
}

// mergeTopK は worker ローカルの topK 群を1つの top-k に畳み込む。
// チャンクは互いに素なので ID の重複は無い。
func mergeTopK(k int, tops []*topK) []Result {
	m := newTopK(k)
	for _, t := range tops {
		if t == nil {
			continue
		}
		for _, r := range t.results() {
			m.push(r.ID, r.Score)
		}
	}
	return m.results()
}
