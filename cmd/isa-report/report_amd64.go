//go:build goexperiment.simd && amd64

package main

import (
	"fmt"
	"sort"

	"simd/archsimd"
)

const docURL = "https://pkg.go.dev/simd/archsimd"

// feature maps a pkg.go.dev "CPU Feature" label to archsimd.X86.
type feature struct {
	name  string
	check func() bool
}

var features = []feature{
	{"AVX", archsimd.X86.AVX},
	{"AVX2", archsimd.X86.AVX2},
	{"FMA", archsimd.X86.FMA},
	{"AVX512", archsimd.X86.AVX512},
	{"AVX512VPOPCNTDQ", archsimd.X86.AVX512VPOPCNTDQ},
}

// api is one archsimd (or related) operation used in this repo.
type api struct {
	stage   string
	symbol  string
	asm     string
	need    string // CPU Feature from pkg.go.dev
	check   func() bool
}

var apis = []api{
	// Stage 0 — scalar, no archsimd
	{"Stage 0", "vec.DotNaive (scalar loop)", "ADDSS/MULSS", "(scalar SSE, not archsimd)", func() bool { return true }},
	{"Stage 0", "vec.Hamming → bits.OnesCount64", "POPCNT", "(scalar POPCNT, not archsimd)", func() bool { return true }},

	// Stage 1 — Float32x8 dot
	{"Stage 1", "LoadFloat32x8Slice", "VMOVDQU", "AVX2", archsimd.X86.AVX2},
	{"Stage 1", "Float32x8.MulAdd", "VFMADD213PS", "FMA", archsimd.X86.FMA},
	{"Stage 1", "archsimd.ClearAVXUpperBits", "VZEROUPPER", "AVX", archsimd.X86.AVX},

	// Stage 2 — binary search (scalar path in production)
	{"Stage 2", "vec.Hamming (same as Stage 0)", "POPCNT", "(scalar)", func() bool { return true }},

	// Bonus — Uint64x4 Hamming SIMD
	{"Bonus", "LoadUint64x4Slice", "VMOVDQU", "AVX2", archsimd.X86.AVX2},
	{"Bonus", "Uint64x4.Xor", "VPXOR", "AVX2", archsimd.X86.AVX2},
	{"Bonus", "Uint64x4.OnesCount", "VPOPCNTQ", "AVX512VPOPCNTDQ", archsimd.X86.AVX512VPOPCNTDQ},
	{"Bonus", "archsimd.ClearAVXUpperBits", "VZEROUPPER", "AVX", archsimd.X86.AVX},

	// Finish — rerank calls vec.Dot (Stage 1 guard)
	{"Finish", "vec.Dot in SearchBinaryRerank", "(Stage 1 APIs)", "AVX2+FMA", func() bool {
		return archsimd.X86.AVX2() && archsimd.X86.FMA()
	}},
}

type stageSummary struct {
	name   string
	guard  string
	active string
}

func run() {
	fmt.Printf("=== archsimd.X86 on this CPU ===\n")
	fmt.Printf("(see %s#CPU-feature-checks)\n\n", docURL)
	for _, f := range features {
		ok := f.check()
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Printf("  %-20s %s %v\n", f.name, mark, ok)
	}

	hasSIMD := archsimd.X86.AVX2() && archsimd.X86.FMA()
	hasVPOPCNT := archsimd.X86.AVX512() && archsimd.X86.AVX512VPOPCNTDQ()
	fmt.Printf("\n  %-20s   %v  (repo guard: dot_simd.go)\n", "HasSIMD", hasSIMD)
	fmt.Printf("  %-20s   %v  (repo guard: hamming_simd.go)\n", "HasVPOPCNT", hasVPOPCNT)

	fmt.Printf("\n=== APIs used in this repo (from pkg.go.dev CPU Feature) ===\n")
	byStage := map[string][]api{}
	for _, a := range apis {
		byStage[a.stage] = append(byStage[a.stage], a)
	}
	stages := sortedKeys(byStage)
	for _, st := range stages {
		fmt.Printf("\n[%s]\n", st)
		for _, a := range byStage[st] {
			ok := a.check()
			mark := "✓"
			if !ok {
				mark = "✗"
			}
			fmt.Printf("  %s %s\n", mark, a.symbol)
			fmt.Printf("      Asm: %s  need: %s  here: %v\n", a.asm, a.need, ok)
		}
	}

	fmt.Printf("\n=== Stage summary (repo guards) ===\n")
	for _, s := range stageSummaries(hasSIMD, hasVPOPCNT) {
		fmt.Printf("\n[%s]\n", s.name)
		fmt.Printf("  guard: %s\n", s.guard)
		fmt.Printf("  active: %s\n", s.active)
	}

	fmt.Printf("\n=== Notes ===\n")
	fmt.Printf("  • Feature checks: archsimd.X86.* — same API pkg.go.dev recommends.\n")
	fmt.Printf("  • Apple Silicon: run with GOARCH=amd64 (Rosetta). AVX-512 unsupported;\n")
	fmt.Printf("    FMA often false → HasSIMD=false. See Apple Rosetta docs.\n")
	fmt.Printf("  • Docker linux/amd64 on Apple Silicon emulates x86; not a substitute for\n")
	fmt.Printf("    amd64 bare metal (Codespaces / make remote-bench).\n")
}

func stageSummaries(hasSIMD, hasVPOPCNT bool) []stageSummary {
	rerank := "partial (binary OK; DotNaive rerank)"
	if hasSIMD {
		rerank = "yes (SIMD rerank)"
	}
	dot := "no → DotNaive fallback"
	if hasSIMD {
		dot = "yes"
	}
	bonus := "no → Hamming fallback"
	if hasVPOPCNT {
		bonus = "yes"
	}
	return []stageSummary{
		{"Stage 0: scalar baseline", "(always)", "yes"},
		{"Stage 1: SIMD dot", "archsimd.X86.AVX2() && FMA()", dot},
		{"Stage 2: binary quantization", "scalar Hamming (POPCNT)", "yes"},
		{"Bonus: AVX-512 Hamming", "AVX512() && AVX512VPOPCNTDQ()", bonus},
		{"Finish: binary + rerank", "Hamming + Dot guard", rerank},
	}
}

func sortedKeys(m map[string][]api) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	order := map[string]int{"Stage 0": 0, "Stage 1": 1, "Stage 2": 2, "Bonus": 3, "Finish": 4}
	sort.Slice(keys, func(i, j int) bool {
		oi, oj := order[keys[i]], order[keys[j]]
		if oi != oj {
			return oi < oj
		}
		return keys[i] < keys[j]
	})
	return keys
}
