// Package index は最小限の全探索ベクトル検索エンジン。
// 高速化の対象は距離計算(internal/vec)で、Index 自体は全ステージ共通。
package index

import (
	"fmt"

	"github.com/po3rin/gocon2026-simd-search/internal/vec"
)

// Result は検索結果の 1 件。Score は大きいほど良い。
type Result struct {
	ID    int
	Score float32
}

// Index はベクトルを 3 つの表現で持つ。
// float32(正確・重い)、1bit 表現(粗い・1/32 サイズ)、
// int8 表現(Stage 2。1/4 サイズ。BuildInt8 で構築)。
type Index struct {
	Dim   int
	Words int
	N     int
	Data  []float32 // N*Dim。行優先
	Codes []uint64  // N*Words。符号 1bit に量子化したもの

	// Stage 2(int8 量子化)。BuildInt8() を呼ぶまで空。
	Codes8 []int8    // N*Dim。対称型のスカラ量子化
	Scales []float32 // N。ベクトルごとの scale(復元は q8*scale ≈ fp32)
}

// New は dim 次元の空の索引を作る。
func New(dim int) *Index {
	return &Index{Dim: dim, Words: vec.Words(dim)}
}

// Add はベクトルを追加し、1bit 表現も同時に作る。
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

// Vec は id 番目の float32 ベクトルを返す。
func (ix *Index) Vec(id int) []float32 {
	return ix.Data[id*ix.Dim : (id+1)*ix.Dim]
}

// Code は id 番目の 1bit 表現を返す。
func (ix *Index) Code(id int) []uint64 {
	return ix.Codes[id*ix.Words : (id+1)*ix.Words]
}

// SearchNaive は Stage 0 の基準実装。全ベクトルとスカラ内積を取る。
func (ix *Index) SearchNaive(q []float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, vec.DotNaive(q, ix.Vec(id)))
	}
	return t.results()
}

// SearchSIMD は Stage 1。走査は SearchNaive と同じで、内積だけ SIMD 版。
func (ix *Index) SearchSIMD(q []float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, vec.Dot(q, ix.Vec(id)))
	}
	return t.results()
}

// SearchPortable は Stage 1 をポータブル simd パッケージ(Go 1.27 の simd.Float32s)で
// 書いたもの。SearchSIMD と同じ走査で、内積だけ vec.DotPortable
// (make bench-portable。workshop.md §02「ポータブルな simd パッケージ」)。
func (ix *Index) SearchPortable(q []float32, k int) []Result {
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, vec.DotPortable(q, ix.Vec(id)))
	}
	return t.results()
}

// SearchBinary は Stage 3。1bit 表現をハミング距離で走査する。
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

// SearchBinarySIMD は付録のパス(本編フロー外)。AVX-512 VPOPCNT でハミング距離を取る。
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

// SearchBinaryRerank は安い 1bit 走査で k*factor 件の候補を出し、
// その候補だけを正確な float32 内積で採点し直す。
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

// SearchBatchNaive は DB を 1 周する間に len(qs) 本のクエリをまとめて処理する。
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

// SearchBatchSIMD は SearchBatchNaive の内積を SIMD 版にしたもの。
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
