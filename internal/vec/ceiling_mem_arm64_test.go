//go:build goexperiment.simd && arm64

package vec

import (
	"testing"

	"simd/archsimd"
)

// メモリ天井(Neon 版)。ceiling_mem_simd_test.go(AVX2 版)の arm64 版で、
// 検索の内積(dot_arm64.go)と同じ 128bit ロードで単コアが DRAM から引ける帯域を測る。
// Apple Silicon はスカラ縮約だと帯域を大きく過小評価する(M3 Pro 実測: スカラ 10.7 GB/s に
// 対して SIMD 全探索が 33 GB/s を達成してしまい、点が屋根を突き抜ける)ので、
// 天井ベンチも検索と同じ SIMD ロードで測る。詳細は ceiling_mem_test.go と
// Go 1.27 の arm64(Neon)対応で追加。

// BenchmarkPeakReadBW は読み取り専用の逐次ストリーム帯域を Neon で測る。
//
// 素直な「ロードして足すだけ」のループは M3 Pro では過小評価になる(実測: 8本 Add で
// 26 GB/s、4本 FMLA で 23 GB/s。いずれも Go のコード生成がアキュムレータをスタックへ
// 退避する spill が挟まる)のに対し、検索の内積 Dot(dot_arm64.go: 4本 FMLA・spill なし)
// で 1536 byte の行を順に流すと 34 GB/s に達し、SIMD 全探索の達成帯域(33 GB/s)と一致する。
// 天井が達成点を下回ると図が破綻するので、ここでは検索と同じ読み方(行ごとに内積)で
// DRAM を流し、その帯域を read 天井とする(q は L1 常駐なので DRAM 転送に数えない)。
func BenchmarkPeakReadBW(b *testing.B) {
	memSetup()
	b.SetBytes(int64(memN) * 4)
	q := memB[:384] // クエリ相当(L1 に乗る)
	var sink float32
	iters := 0
	for b.Loop() {
		x := memB
		var tot float32
		for len(x) >= 384 {
			tot += Dot(q, x[:384])
			x = x[384:]
		}
		sink += tot
		iters++
	}
	ceilSinkFloat = sink
	sec := b.Elapsed().Seconds()
	gb := float64(memN) * 4 * float64(iters) / sec / 1e9
	b.ReportMetric(gb, "read-GB/s")
}

// BenchmarkPeakTriadBW は STREAM Triad(a = b + s*c)を Neon で測る。
// MulAdd(FMLA)1発で c*s+b を計算して store。1要素あたり read b + read c +
// write a = 3 配列 ×4byte の論理転送(STREAM 慣習)。
func BenchmarkPeakTriadBW(b *testing.B) {
	memSetup()
	b.SetBytes(int64(memN) * 4 * 3)
	s := archsimd.BroadcastFloat32x4(3.0)
	iters := 0
	for b.Loop() {
		aa, bb, cc := memA, memB, memC
		for len(aa) >= 16 {
			archsimd.LoadFloat32x4(cc).MulAdd(s, archsimd.LoadFloat32x4(bb)).Store(aa)
			archsimd.LoadFloat32x4(cc[4:]).MulAdd(s, archsimd.LoadFloat32x4(bb[4:])).Store(aa[4:])
			archsimd.LoadFloat32x4(cc[8:]).MulAdd(s, archsimd.LoadFloat32x4(bb[8:])).Store(aa[8:])
			archsimd.LoadFloat32x4(cc[12:]).MulAdd(s, archsimd.LoadFloat32x4(bb[12:])).Store(aa[12:])
			aa, bb, cc = aa[16:], bb[16:], cc[16:]
		}
		iters++
	}
	ceilSinkFloat = memA[memN-1]
	sec := b.Elapsed().Seconds()
	gb := float64(memN) * 4 * 3 * float64(iters) / sec / 1e9
	b.ReportMetric(gb, "triad-GB/s")
}
