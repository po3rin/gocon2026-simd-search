package index

import "github.com/po3rin/gocon2026-simd-search/internal/vec"

// BuildInt8 は全ベクトルの int8 表現(1/4 サイズ)を作る(Stage 3)。
// 1bit と違って大きさの情報が残るので、rerank なしでも Recall が実用域に残る。
func (ix *Index) BuildInt8() {
	ix.Codes8 = make([]int8, ix.N*ix.Dim)
	ix.Scales = make([]float32, ix.N)
	for id := 0; id < ix.N; id++ {
		ix.Scales[id] = vec.QuantizeInt8(ix.Vec(id), ix.Codes8[id*ix.Dim:(id+1)*ix.Dim])
	}
}

// Code8 は id 番目の int8 表現を返す。
func (ix *Index) Code8(id int) []int8 {
	return ix.Codes8[id*ix.Dim : (id+1)*ix.Dim]
}

// SearchInt8 は Stage 3。int8 表現で全探索する。先に BuildInt8 を呼んでおくこと。
// スコアは qScale*dScale*dot_int8 で、fp32 の内積の近似になる。
// 1 ベクトルの転送は 384 byte(fp32 の 1/4)なので算術強度が 4 倍に上がる。
func (ix *Index) SearchInt8(q []float32, k int) []Result {
	if ix.Codes8 == nil {
		panic("index: SearchInt8 の前に BuildInt8 を呼んでください")
	}
	q8 := make([]int8, ix.Dim)
	qScale := vec.QuantizeInt8(q, q8)
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, qScale*ix.Scales[id]*float32(vec.DotInt8(q8, ix.Code8(id))))
	}
	return t.results()
}
