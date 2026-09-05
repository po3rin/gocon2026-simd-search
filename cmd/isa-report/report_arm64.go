//go:build goexperiment.simd && arm64

package main

import (
	"fmt"
	"runtime"

	"simd"
)

const docURL = "https://pkg.go.dev/simd/archsimd"

// api is one archsimd (or related) operation used by the arm64 kernels in this repo.
// Go 1.27 の archsimd は arm64 では Neon(128bit)のみ。Neon は ARMv8-A の必須機能
// なので、amd64 のような CPU 機能チェック(archsimd.X86.*)は arm64 には無い
// (archsimd.ARM64 にあるのは PMULL だけ)。
type api struct {
	stage  string
	symbol string
	asm    string
}

var apis = []api{
	{"Stage 0", "vec.DotNaive (scalar loop)", "FMUL/FADD (scalar)"},
	{"Stage 0", "vec.Hamming → bits.OnesCount64", "CNT + ADDV (Go intrinsic)"},
	{"Stage 1", "LoadFloat32x4", "LDR Q / VLD1"},
	{"Stage 1", "Float32x4.MulAdd", "FMLA"},
	{"Stage 1", "Float32x4.Store", "STR Q / VST1"},
	{"Stage 3", "LoadInt8x16", "LDR Q / VLD1"},
	{"Stage 3", "Int8x16.MulWidenLo", "SMULL"},
	{"Stage 3", "Int8x16.HiToLo", "EXT"},
	{"Stage 3", "Int16x8.ExtendLo4ToInt32", "SXTL"},
	{"Stage 3", "Int32x4.ReduceSum", "ADDV"},
	{"Stage 4", "vec.Hamming (same as Stage 0)", "CNT + ADDV"},
	{"Stage 5", "vec.Dot in SearchBinaryRerank", "(Stage 1 APIs)"},
}

func run() {
	fmt.Printf("=== archsimd on this CPU (%s/%s) ===\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("(see %s)\n\n", docURL)
	fmt.Printf("  %-20s ✓ true   (Neon 128bit は ARMv8-A 必須。機能チェック不要)\n", "NEON")
	fmt.Printf("  %-20s   %v  (repo guard: dot_arm64.go / int8_arm64.go)\n", "HasSIMD", true)
	fmt.Printf("  %-20s   %v  (AVX-512 専用の付録B。arm64 ではスカラ Hamming)\n", "HasVPOPCNT", false)

	fmt.Printf("\n=== portable simd package (Go 1.27) ===\n")
	fmt.Printf("  %-20s   %d bit  (simd.Float32s = %d lanes)\n", "VectorBitSize", simd.VectorBitSize(), simd.Float32s{}.Len())
	fmt.Printf("  %-20s   %v\n", "Emulated", simd.Emulated())

	fmt.Printf("\n=== APIs used by the arm64 kernels in this repo ===\n")
	last := ""
	for _, a := range apis {
		if a.stage != last {
			fmt.Printf("\n[%s]\n", a.stage)
			last = a.stage
		}
		fmt.Printf("  ✓ %s\n      Asm: %s\n", a.symbol, a.asm)
	}

	fmt.Printf("\n=== Notes ===\n")
	fmt.Printf("  • 本編(workshop.md)の数字は Codespaces(amd64 / AVX2+FMA・256bit)のもの。\n")
	fmt.Printf("    ここ(Neon 128bit)で出る倍率は「自分の Mac の点」として読む。\n")
	fmt.Printf("  • Neon の archsimd には VPMADDWD(DotProductPairs)や 64bit popcount が無い。\n")
	fmt.Printf("    int8 カーネルは SMULL+SXTL の 3 段、Hamming はスカラ(CNT 内蔵)のまま。\n")
	fmt.Printf("  • amd64 の一覧は: GOARCH=amd64 GOEXPERIMENT=simd go run ./cmd/isa-report/\n")
}
