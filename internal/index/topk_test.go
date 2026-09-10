package index

import "testing"

func TestTopK(t *testing.T) {
	// k=0 と負の k: push しても空のまま(パニックしない)
	for _, k := range []int{0, -1} {
		tk := newTopK(k)
		tk.push(1, 1.0)
		if got := tk.results(); len(got) != 0 {
			t.Errorf("k=%d: got %d results, want 0", k, len(got))
		}
	}

	// k > 件数: ある分だけスコア降順で返る
	tk := newTopK(5)
	tk.push(0, 1.0)
	tk.push(1, 3.0)
	if got := tk.results(); len(got) != 2 || got[0].ID != 1 || got[1].ID != 0 {
		t.Errorf("k>N: got %v", got)
	}

	// 満杯後の入れ替え: 上位 2 件だけ残る
	tk = newTopK(2)
	for id, s := range []float32{1, 5, 3, 4} {
		tk.push(id, s)
	}
	if got := tk.results(); got[0].ID != 1 || got[1].ID != 3 {
		t.Errorf("top-2: got %v, want IDs [1 3]", got)
	}

	// 同点は先着(走査が ID 昇順なら小さい ID)優先
	tk = newTopK(2)
	tk.push(0, 1.0)
	tk.push(1, 1.0)
	tk.push(2, 1.0)
	if got := tk.results(); got[0].ID != 0 || got[1].ID != 1 {
		t.Errorf("ties: got %v, want IDs [0 1]", got)
	}

	// 満杯時に最小値と同点の要素が来ても既存が残る
	tk = newTopK(2)
	tk.push(0, 2.0)
	tk.push(1, 1.0)
	tk.push(2, 1.0)
	if got := tk.results(); got[0].ID != 0 || got[1].ID != 1 {
		t.Errorf("tie with min: got %v, want IDs [0 1]", got)
	}
}
