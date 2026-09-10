package index

import (
	"fmt"
	"math"

	"github.com/po3rin/gocon2026-simd-search/internal/vec"
)

// MultiIndex は文書を「トークンベクトルの集合」で持つ(付録 8 節: late interaction)。
// ColBERT 系の検索方式で、スコアは MaxSim:
//
//	score(q, d) = Σ_{qt∈q} max_{dt∈d} dot(qt, dt)
//
// 文書トークン dt を1回ロードするとクエリトークン全部(Tq 本)と内積するので、
// バッチ化(付録 4 節)と同じ「1ロードに対し多数の計算」がタスクの仕様として内在する
// = 最初から演算律速で、SIMD が最初から効く。
type MultiIndex struct {
	Dim  int
	Tok  int // 文書あたりのトークンベクトル数
	N    int
	Data []float32 // N*Tok*Dim, row-major
}

// NewMulti creates an empty multi-vector index.
func NewMulti(dim, tok int) *MultiIndex {
	return &MultiIndex{Dim: dim, Tok: tok}
}

// Add appends a document (tok 本のトークンベクトル).
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

// DocToken returns document id's t-th token vector.
func (mx *MultiIndex) DocToken(id, t int) []float32 {
	off := (id*mx.Tok + t) * mx.Dim
	return mx.Data[off : off+mx.Dim]
}

// maxSim は1文書ぶんの MaxSim スコアを計算する。dot は内積関数
// (DotNaive / Dot)を差し替えて naive/SIMD を比較する。
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

// SearchMaxSimNaive is MaxSim search with the scalar dot product.
func (mx *MultiIndex) SearchMaxSimNaive(q [][]float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < mx.N; id++ {
		t.push(id, mx.maxSim(q, id, vec.DotNaive))
	}
	return t.results()
}

// SearchMaxSimSIMD is MaxSim search with the SIMD dot product.
func (mx *MultiIndex) SearchMaxSimSIMD(q [][]float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < mx.N; id++ {
		t.push(id, mx.maxSim(q, id, vec.Dot))
	}
	return t.results()
}
