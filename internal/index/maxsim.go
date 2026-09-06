package index

import (
	"fmt"
	"math"

	"github.com/po3rin/gocon2026-simd-search/internal/vec"
)

// MultiIndex は文書を「トークンベクトルの集合」で持つ(付録 2 節、late interaction)。
// ColBERT 系の検索方式で、スコアは MaxSim:
//
//	score(q, d) = Σ_{qt∈q} max_{dt∈d} dot(qt, dt)
//
// 文書トークン dt を 1 回ロードするたびにクエリトークン全部(Tq 本)と内積を取るので、
// バッチ化(Stage 2)と同じ「1 回のロードに多数の計算」が検索方式そのものに含まれる。
// つまり最初から演算律速で、SIMD が最初から効く。
type MultiIndex struct {
	Dim  int
	Tok  int // 文書あたりのトークンベクトル数
	N    int
	Data []float32 // N*Tok*Dim, row-major
}

// NewMulti は 1 文書あたり tok 本の dim 次元ベクトルを持つ空の索引を作る。
func NewMulti(dim, tok int) *MultiIndex {
	return &MultiIndex{Dim: dim, Tok: tok}
}

// Add は文書(tok 本のトークンベクトル)を追加する。
func (mx *MultiIndex) Add(doc [][]float32) {
	if len(doc) != mx.Tok {
		panic(fmt.Sprintf("index: token count mismatch: got %d want %d", len(doc), mx.Tok))
	}
	for _, v := range doc {
		if len(v) != mx.Dim {
			panic(fmt.Sprintf("index: dim mismatch: got %d want %d", len(v), mx.Dim))
		}
		mx.Data = append(mx.Data, v...)
	}
	mx.N++
}

// DocToken は文書 id の t 番目のトークンベクトルを返す。
func (mx *MultiIndex) DocToken(id, t int) []float32 {
	off := (id*mx.Tok + t) * mx.Dim
	return mx.Data[off : off+mx.Dim]
}

// maxSim は 1 文書ぶんの MaxSim スコアを計算する。内積関数 dot を差し替えてスカラと SIMD を比較する。
func (mx *MultiIndex) maxSim(q [][]float32, id int, dot func(a, b []float32) float32) float32 {
	var s float32
	for _, qt := range q {
		best := float32(math.Inf(-1))
		for t := 0; t < mx.Tok; t++ {
			if d := dot(qt, mx.DocToken(id, t)); d > best {
				best = d
			}
		}
		s += best
	}
	return s
}

// SearchMaxSimNaive はスカラの内積で MaxSim 検索する。
func (mx *MultiIndex) SearchMaxSimNaive(q [][]float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < mx.N; id++ {
		t.push(id, mx.maxSim(q, id, vec.DotNaive))
	}
	return t.results()
}

// SearchMaxSimSIMD は SIMD の内積で MaxSim 検索する。
func (mx *MultiIndex) SearchMaxSimSIMD(q [][]float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < mx.N; id++ {
		t.push(id, mx.maxSim(q, id, vec.Dot))
	}
	return t.results()
}
