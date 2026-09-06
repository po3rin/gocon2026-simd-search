package index

import (
	"sync"

	"github.com/po3rin/gocon2026-simd-search/internal/vec"
)

// SearchParallel は SearchSIMD(SIMD 内積の全探索)を workers 本の goroutine で並列化する。
// DB を連続したチャンクに分割し、各 worker が自分のチャンクだけを走査して
// worker ごとの topK を作り、最後にマージする。
//
// 全探索はメモリ律速なので、コアを増やしても DRAM 帯域を取り合うだけで
// workers 倍にはならない(workshop.md §06 の goroutine のコラム)。
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

// SearchBatchParallel は SearchBatchSIMD(B 本のクエリを 1 パスで処理)を workers 本の
// goroutine で並列化する。分割は SearchParallel と同じ DB のチャンク分割で、
// worker ごとの算術強度は 0.5×B のまま変わらない。
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

// mergeTopK は worker ごとの topK を 1 つにまとめる。チャンクは互いに素なので ID は重複しない。
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
