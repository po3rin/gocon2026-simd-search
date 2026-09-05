// Command roofline-decompose emits an SVG that splits per-element time into
// "memory time" (bytes moved / bandwidth) and "compute time" (flop / peak),
// for the full-scan (B=1, AI=0.5) and batched (B=32, AI=16) dot-product
// search. It shows how batching shrinks only the memory time, flipping the
// bottleneck from memory-bound to compute-bound.
//
// Inputs are the machine's measured ceilings (from `make roofline-ceiling`):
//
//	go run ./cmd/roofline-decompose -peak 25.51 -bw 18.39 > out.svg
//
// Or: make roofline-decompose  (regenerates docs/images/memory-vs-compute-roofline.svg)
//
// The split is the roofline MODEL's prediction (actual time is slower due to
// register spill etc.); CPU profilers (pprof/trace) cannot produce it.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// per-element kernel cost of the inner product a·d
const (
	flopPerElem = 2.0 // 1 mul + 1 add
	bytePerElem = 4.0 // one fp32 DB element
)

type stage struct {
	title string
	ai    string
	bytes float64 // bytes moved per element (amortized by batching)
	y     float64
}

func main() {
	peak := flag.Float64("peak", 25.51, "compute ceiling in GFLOP/s (from make roofline-ceiling)")
	bw := flag.Float64("bw", 18.39, "memory read bandwidth in GB/s (from make roofline-ceiling)")
	flag.Parse()

	stages := []stage{
		{"全探索 (B=1)", "算術強度=0.5", bytePerElem, 130},      // no reuse
		{"バッチ (B=32)", "算術強度=16", bytePerElem / 32, 280}, // d loaded once for 32 queries
	}

	tc := flopPerElem / *peak // compute time per element (same for every stage)
	k := 150.0 / tc           // scale so the compute bar is 150px wide

	const x0, hb = 240.0, 30.0
	memFill, memStroke, memText := "#dbeafe", "#60a5fa", "#1e40af"
	cFill, cStroke, cText := "#ffedd5", "#fb923c", "#9a3412"

	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	p(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 440">` + "\n")
	p(`<style>text{font-family:'Helvetica Neue',Helvetica,Arial,'Hiragino Sans','Hiragino Kaku Gothic ProN',sans-serif;}</style>` + "\n")
	p(`<rect width="960" height="440" fill="#ffffff"/>` + "\n")
	p(`<text x="480" y="32" font-size="18" font-weight="600" fill="#111827" text-anchor="middle">メモリ時間 vs 演算時間 — バッチ化で律速が反転(ルーフライン分解)</text>` + "\n")
	p(`<text x="480" y="54" font-size="11.5" fill="#6b7280" text-anchor="middle">Go 実測の上限(演算 %.1f GFLOP/s、read 帯域 %.1f GB/s。make roofline-ceiling)から計算。実時間はおおよそ長い方。</text>`+"\n", *peak, *bw)

	for _, s := range stages {
		tm := s.bytes / *bw
		mw, cw := tm*k, tc*k
		p(`<text x="120" y="%.0f" font-size="13.5" font-weight="600" fill="#111827" text-anchor="middle">%s</text>`+"\n", s.y+24, s.title)
		p(`<text x="120" y="%.0f" font-size="11" fill="#6b7280" text-anchor="middle">%s</text>`+"\n", s.y+42, s.ai)

		// memory-time bar
		p(`<rect x="%.0f" y="%.0f" width="%.1f" height="%.0f" fill="%s" stroke="%s" stroke-width="1.6"/>`+"\n", x0, s.y, mw, hb, memFill, memStroke)
		if mw > 180 {
			p(`<text x="%.1f" y="%.1f" font-size="12" fill="%s" text-anchor="middle">メモリ時間 (運ぶbyte / 帯域)</text>`+"\n", x0+mw/2, s.y+hb/2+5, memText)
		} else {
			p(`<text x="%.1f" y="%.1f" font-size="12" fill="#6b7280" text-anchor="start">メモリ時間</text>`+"\n", x0+mw+8, s.y+hb/2+5)
		}

		// compute-time bar
		yc := s.y + hb + 8
		p(`<rect x="%.0f" y="%.1f" width="%.1f" height="%.0f" fill="%s" stroke="%s" stroke-width="1.6"/>`+"\n", x0, yc, cw, hb, cFill, cStroke)
		p(`<text x="%.1f" y="%.1f" font-size="12" fill="%s" text-anchor="middle">演算時間 (2flop / ピーク)</text>`+"\n", x0+cw/2, yc+hb/2+5, cText)

		// bottleneck = the longer bar
		if mw > cw {
			p(`<text x="%.1f" y="%.1f" font-size="12" font-weight="600" fill="%s" text-anchor="start">← 律速: メモリ</text>`+"\n", x0+mw+10, s.y+hb/2+5, memText)
		} else {
			p(`<text x="%.1f" y="%.1f" font-size="12" font-weight="600" fill="%s" text-anchor="start">← 律速: 演算</text>`+"\n", x0+cw+10, yc+hb/2+5, cText)
		}
	}

	p(`<text x="480" y="372" font-size="12.5" font-weight="600" fill="#111827" text-anchor="middle">バッチ化(B=32)で「演算時間」は不変、「メモリ時間」だけ 1/32 に縮む → 律速がメモリ→演算へ反転</text>` + "\n")
	p(`<rect x="300" y="392" width="14" height="14" rx="3" fill="%s" stroke="%s" stroke-width="1.5"/>`+"\n", memFill, memStroke)
	p(`<text x="320" y="403" font-size="11.5" fill="#374151">メモリ時間 = 運ぶbyte / 帯域</text>` + "\n")
	p(`<rect x="560" y="392" width="14" height="14" rx="3" fill="%s" stroke="%s" stroke-width="1.5"/>`+"\n", cFill, cStroke)
	p(`<text x="580" y="403" font-size="11.5" fill="#374151">演算時間 = flop / 演算ピーク</text>` + "\n")
	p(`<text x="480" y="428" font-size="10.5" fill="#9ca3af" text-anchor="middle">※ ルーフラインの理想値。実測は spill 等でこれより遅い(本文)。この内訳は pprof/trace では出せず、ルーフラインが与える。</text>` + "\n")
	p(`</svg>` + "\n")

	os.Stdout.WriteString(b.String())
}
