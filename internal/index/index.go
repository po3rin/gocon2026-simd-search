// Package index implements a minimal brute-force vector search engine.
// 高速化の対象は距離計算(internal/vec)で、Index 自体は全ステージ共通。
package index

import (
	"fmt"

	"github.com/po3rin/gocon2026-simd-search/internal/vec"
)

// Result is a single search hit. Score is higher-is-better.
type Result struct {
	ID    int
	Score float32
}

// Index holds the vectors in multiple representations:
// float32(正確・重い)、binary code(粗い・1/32 サイズ)、
// int8 code(Stage 2: 1/4 サイズ・BuildInt8 で構築)。
type Index struct {
	Dim   int
	Words int
	N     int
	Data  []float32 // N*Dim, row-major
	Codes []uint64  // N*Words, sign-bit quantized

	// Stage 2(int8 量子化)。BuildInt8() を呼ぶまで空。
	Codes8 []int8    // N*Dim, symmetric int8 quantized
	Scales []float32 // N, per-vector scale(復元は q8*scale ≈ fp32)
}

// New creates an empty index for dim-dimensional vectors.
func New(dim int) *Index {
	return &Index{Dim: dim, Words: vec.Words(dim)}
}

// Add appends a vector and its binary code to the index.
func (ix *Index) Add(v []float32) {
	if len(v) != ix.Dim {
		panic(fmt.Sprintf("index: dim mismatch: got %d want %d", len(v), ix.Dim))
	}
	ix.Data = append(ix.Data, v...)
	code := make([]uint64, ix.Words)
	vec.Quantize(v, code)
	ix.Codes = append(ix.Codes, code...)
	ix.N++
	// int8 表現はベースが変わると古くなるので捨てる(次の BuildInt8 で作り直す)
	ix.Codes8, ix.Scales = nil, nil
}

// Vec returns the float32 vector for id.
func (ix *Index) Vec(id int) []float32 {
	return ix.Data[id*ix.Dim : (id+1)*ix.Dim]
}

// Code returns the binary code for id.
func (ix *Index) Code(id int) []uint64 {
	return ix.Codes[id*ix.Words : (id+1)*ix.Words]
}

// SearchNaive is the Stage 0 baseline: scalar dot product over all vectors.
func (ix *Index) SearchNaive(q []float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, vec.DotNaive(q, ix.Vec(id)))
	}
	return t.results()
}

// SearchSIMD is Stage 1: same scan, SIMD dot product.
func (ix *Index) SearchSIMD(q []float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, vec.Dot(q, ix.Vec(id)))
	}
	return t.results()
}

// SearchPortable is Stage 1 written with the portable simd package
// (Go 1.27 の simd.Float32s)。SearchSIMD と同じ走査で、内積だけ vec.DotPortable
// (make bench-portable。workshop.md §02「ポータブルな simd パッケージ」)。
func (ix *Index) SearchPortable(q []float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, vec.DotPortable(q, ix.Vec(id)))
	}
	return t.results()
}

// SearchBinary is Stage 3: scan over binary codes with Hamming distance.
// Score は -距離(距離が小さいほど良い)。
func (ix *Index) SearchBinary(q []float32, k int) []Result {
	code := make([]uint64, ix.Words)
	vec.Quantize(q, code)
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, -float32(vec.Hamming(code, ix.Code(id))))
	}
	return t.results()
}

// SearchBinarySIMD is an appendix path (本編フロー外): Hamming with AVX-512 VPOPCNT.
// 量子化後はキャッシュ律速で popcount を SIMD 化しても速くならない(計測上 SearchBinarySIMD
// ≧ SearchBinary)ため、本編 Stage には含めず付録として残置。AVX-512 機向け(make bench-bonus)。
// 非対応 CPU では vec.HammingSIMD がスカラ Hamming にフォールバックする。
func (ix *Index) SearchBinarySIMD(q []float32, k int) []Result {
	code := make([]uint64, ix.Words)
	vec.Quantize(q, code)
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, -float32(vec.HammingSIMD(code, ix.Code(id))))
	}
	return t.results()
}

// SearchBinaryRerank retrieves k*factor candidates with the cheap binary
// scan, then re-scores them with the exact float32 dot product.
// 「速度と精度は二者択一ではない」を示す QBit 風の二段構え。
// 戻り値の Score は SearchBinary と違い、-Hamming ではなく fp32 の内積。
func (ix *Index) SearchBinaryRerank(q []float32, k, factor int) []Result {
	cands := ix.SearchBinary(q, k*factor)
	t := newTopK(k)
	for _, c := range cands {
		t.push(c.ID, vec.Dot(q, ix.Vec(c.ID)))
	}
	return t.results()
}

// SearchBatchNaive runs len(qs) queries in a single pass over the DB.
// 各 DB ベクトル d を1回ロードして B 本のクエリ全部と内積する(d はキャッシュ常駐で
// 使い回される)。DRAM 転送は B=1 と同じなので算術強度 AI ≈ 0.5×B に上がり、
// バッチを大きくするほど演算律速側へ移る(事実上の GEMM 化)。
func (ix *Index) SearchBatchNaive(qs [][]float32, k int) [][]Result {
	tops := make([]*topK, len(qs))
	for b := range tops {
		tops[b] = newTopK(k)
	}
	for id := 0; id < ix.N; id++ {
		d := ix.Vec(id)
		for b := range qs {
			tops[b].push(id, vec.DotNaive(qs[b], d))
		}
	}
	out := make([][]Result, len(qs))
	for b := range tops {
		out[b] = tops[b].results()
	}
	return out
}

// SearchBatchSIMD is SearchBatchNaive with the SIMD dot.
// バッチで演算律速にした上で SIMD を効かせる狙い。
func (ix *Index) SearchBatchSIMD(qs [][]float32, k int) [][]Result {
	tops := make([]*topK, len(qs))
	for b := range tops {
		tops[b] = newTopK(k)
	}
	for id := 0; id < ix.N; id++ {
		d := ix.Vec(id)
		for b := range qs {
			tops[b].push(id, vec.Dot(qs[b], d))
		}
	}
	out := make([][]Result, len(qs))
	for b := range tops {
		out[b] = tops[b].results()
	}
	return out
}
