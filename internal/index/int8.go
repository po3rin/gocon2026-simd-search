package index

import "github.com/po3rin/gocon2026-simd-search/internal/vec"

// BuildInt8 は全ベクトルの int8 表現(1/4 サイズ)を構築する(付録A)。
// binary(1/32)と違い大きさの情報が残るので、rerank なしでも Recall が実用域。
func (ix *Index) BuildInt8() {
	ix.Codes8 = make([]int8, ix.N*ix.Dim)
	ix.Scales = make([]float32, ix.N)
	for id := 0; id < ix.N; id++ {
		ix.Scales[id] = vec.QuantizeInt8(ix.Vec(id), ix.Codes8[id*ix.Dim:(id+1)*ix.Dim])
	}
}

// Code8 returns the int8 code for id.
func (ix *Index) Code8(id int) []int8 {
	return ix.Codes8[id*ix.Dim : (id+1)*ix.Dim]
}

// SearchInt8 は int8 量子化した表現で全探索する(付録A)。
// スコアは qScale*dScale*dot_int8 ≈ fp32 の内積(近似)。
// 転送は 1ベクトル 384 byte(fp32 の 1/4)→ AI が4倍に上がる。
func (ix *Index) SearchInt8(q []float32, k int) []Result {
	if ix.Codes8 == nil {
		ix.BuildInt8()
	}
	q8 := make([]int8, ix.Dim)
	qScale := vec.QuantizeInt8(q, q8)
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, qScale*ix.Scales[id]*float32(vec.DotInt8(q8, ix.Code8(id))))
	}
	return t.results()
}
