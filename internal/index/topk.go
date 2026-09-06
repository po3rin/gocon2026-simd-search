package index

// topK はこれまでに見た中でスコア上位 k 件を、スコアの降順で保持する。
// k は小さい(100 程度まで)前提の挿入ソート。
type topK struct {
	k  int
	rs []Result
}

func newTopK(k int) *topK {
	return &topK{k: k, rs: make([]Result, 0, k)}
}

func (t *topK) push(id int, score float32) {
	if len(t.rs) == t.k {
		if score <= t.rs[t.k-1].Score {
			return
		}
		t.rs = t.rs[:t.k-1]
	}
	i := len(t.rs)
	t.rs = append(t.rs, Result{})
	for i > 0 && t.rs[i-1].Score < score {
		t.rs[i] = t.rs[i-1]
		i--
	}
	t.rs[i] = Result{ID: id, Score: score}
}

func (t *topK) results() []Result { return t.rs }
