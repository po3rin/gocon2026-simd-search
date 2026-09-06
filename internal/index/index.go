// Package index は最小限の全探索ベクトル検索エンジン。
// 高速化の対象は距離カーネル(internal/vec)で、Index 自体は全 Stage で共通。
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
//   - Data:   float32(正確。重い)
//   - Codes:  符号 1bit(Stage 4。1/32 サイズ。Add で同時に作る)
//   - Codes8: int8(Stage 3。1/4 サイズ。BuildInt8 で作る)
type Index struct {
	Dim   int
	Words int // 1 ベクトルの 1bit 表現に要る uint64 の語数
	N     int
	Data  []float32 // N*Dim、行優先
	Codes []uint64  // N*Words

	Codes8 []int8    // N*Dim。BuildInt8 を呼ぶまで nil
	Scales []float32 // N。ベクトルごとの scale(q8*scale で fp32 に戻る)
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
}

// Vec は id 番目の float32 ベクトルを返す。
func (ix *Index) Vec(id int) []float32 {
	return ix.Data[id*ix.Dim : (id+1)*ix.Dim]
}

// Code は id 番目の 1bit 表現を返す。
func (ix *Index) Code(id int) []uint64 {
	return ix.Codes[id*ix.Words : (id+1)*ix.Words]
}

// scan は全ベクトルを走査し、内積関数 dot で採点して上位 k 件を返す。
// Stage 0 と Stage 1 の違いは dot だけで、走査の形は同じ。
func (ix *Index) scan(q []float32, k int, dot func(a, b []float32) float32) []Result {
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, dot(q, ix.Vec(id)))
	}
	return t.results()
}

// SearchNaive は Stage 0。スカラの内積で全探索する。
func (ix *Index) SearchNaive(q []float32, k int) []Result { return ix.scan(q, k, vec.DotNaive) }

// SearchSIMD は Stage 1。SIMD の内積で全探索する。
func (ix *Index) SearchSIMD(q []float32, k int) []Result { return ix.scan(q, k, vec.Dot) }

// SearchPortable は Stage 1 をポータブル simd パッケージで書いた版(workshop.md Stage 1 コラム)。
// GODEBUG=simd=128 で幅を狭めると点がどこへ動くかを見る(make bench-portable)。
func (ix *Index) SearchPortable(q []float32, k int) []Result { return ix.scan(q, k, vec.DotPortable) }

// scanBinary は 1bit 表現を走査し、ハミング距離で採点して上位 k 件を返す。
// Score は距離の符号を反転したもの(距離が小さいほど良い)。
func (ix *Index) scanBinary(q []float32, k int, hamming func(a, b []uint64) int) []Result {
	code := make([]uint64, ix.Words)
	vec.Quantize(q, code)
	t := newTopK(k)
	for id := 0; id < ix.N; id++ {
		t.push(id, -float32(hamming(code, ix.Code(id))))
	}
	return t.results()
}

// SearchBinary は Stage 4。1bit 表現とスカラの POPCNT で全探索する。
func (ix *Index) SearchBinary(q []float32, k int) []Result { return ix.scanBinary(q, k, vec.Hamming) }

// SearchBinarySIMD は付録 3 節(本編フロー外)。popcount を AVX-512 VPOPCNT で SIMD 化しても
// 速くならないことの確認用(make bench-bonus)。非対応 CPU では vec.HammingSIMD がスカラ版に落ちる。
func (ix *Index) SearchBinarySIMD(q []float32, k int) []Result {
	return ix.scanBinary(q, k, vec.HammingSIMD)
}

// SearchBinaryRerank は Stage 5。1bit で k*factor 件まで粗く絞り、
// その候補だけを fp32 の SIMD 内積で採点し直して上位 k 件を返す。
func (ix *Index) SearchBinaryRerank(q []float32, k, factor int) []Result {
	cands := ix.SearchBinary(q, k*factor)
	t := newTopK(k)
	for _, c := range cands {
		t.push(c.ID, vec.Dot(q, ix.Vec(c.ID)))
	}
	return t.results()
}

// scanBatch は B 本のクエリを 1 パスで処理する。DB ベクトル d を 1 回ロードするたびに
// B 本のクエリ全部と内積を取る(d はキャッシュに乗ったまま使い回される)。
// DRAM から運ぶ量は B=1 と同じなので、算術強度が 0.5×B に上がり演算律速側へ移る。
func (ix *Index) scanBatch(qs [][]float32, k int, dot func(a, b []float32) float32) [][]Result {
	tops := make([]*topK, len(qs))
	for b := range tops {
		tops[b] = newTopK(k)
	}
	for id := 0; id < ix.N; id++ {
		d := ix.Vec(id)
		for b := range qs {
			tops[b].push(id, dot(qs[b], d))
		}
	}
	out := make([][]Result, len(qs))
	for b := range tops {
		out[b] = tops[b].results()
	}
	return out
}

// SearchBatchNaive は Stage 2 の比較用。バッチ化した上でスカラの内積を使う。
func (ix *Index) SearchBatchNaive(qs [][]float32, k int) [][]Result {
	return ix.scanBatch(qs, k, vec.DotNaive)
}

// SearchBatchSIMD は Stage 2。バッチ化した上で SIMD の内積を使う。
func (ix *Index) SearchBatchSIMD(qs [][]float32, k int) [][]Result {
	return ix.scanBatch(qs, k, vec.Dot)
}
