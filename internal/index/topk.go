package index

// topK keeps the k highest-scoring results seen so far,
// sorted by descending score. k は小さい(〜100)前提の挿入ソート。
type topK struct {
	k  int
	rs []Result
}

func newTopK(k int) *topK {
	if k < 0 {
		k = 0
	}
	return &topK{k: k, rs: make([]Result, 0, k)}
}

func (t *topK) push(id int, score float32) {
	if t.k == 0 {
		return
	}
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

// results は内部スライスをそのまま返す。呼び出し側は読み取りのみ想定。
func (t *topK) results() []Result { return t.rs }
