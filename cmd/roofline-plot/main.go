// Command roofline-plot turns `go test -bench` output into an interactive
// roofline HTML: it reads the benchmark lines on stdin, picks the points that
// report both AI(flop/byte) and GFLOP/s, and plots them on a log-log roofline
// whose ceilings come from the machine's measured peak/bandwidth.
//
// The point of the *interactive* version (vs the static docs/images/rl-*.png)
// is that every run re-measures and re-plots: you watch the point appear and
// stick to a ceiling. Stage 1 (AI=0.5) pins to the memory roof; the batched
// Stage 2 (AI=16) crosses the ridge onto the compute roof.
//
// Usage (see `make roofline-plot`):
//
//	go test ./internal/index -run - \
//	  -bench 'BenchmarkSearch(Naive|SIMD|BatchNaive|BatchSIMD)$' -benchtime 2s \
//	  | go run ./cmd/roofline-plot -peak 25.51 -bw 18.39 > roofline.html
//
// Ceilings default to the EPYC 7763 example; pass your own from
// `make roofline-ceiling`. Zero dependencies — same hand-built-SVG style as
// cmd/roofline-decompose. Benchmarks without AI/GFLOP/s (e.g. SearchBinary,
// which uses Hamming distance, not flop) are skipped: they live on a different
// axis and are covered by the static Stage 3 image instead.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// point is one benchmark plotted on the roofline.
type point struct {
	name string  // friendly label
	ai   float64 // arithmetic intensity (flop/byte)
	gf   float64 // achieved GFLOP/s
}

// label maps a raw benchmark name to a friendly stage label.
func label(raw string) string {
	switch raw {
	case "SearchNaive":
		return "Stage 0  scalar (B=1)"
	case "SearchSIMD":
		return "Stage 1  SIMD (B=1)"
	case "SearchBatchNaive":
		return "Stage 2  scalar batch (B=32)"
	case "SearchBatchSIMD":
		return "Stage 2  SIMD batch (B=32)"
	}
	return strings.TrimPrefix(raw, "Search")
}

// parseBench reads `go test -bench` output and returns the points that report
// both AI(flop/byte) and GFLOP/s.
func parseBench(r *bufio.Scanner) []point {
	var pts []point
	for r.Scan() {
		f := strings.Fields(r.Text())
		if len(f) < 4 || !strings.HasPrefix(f[0], "Benchmark") {
			continue
		}
		raw := strings.TrimPrefix(f[0], "Benchmark")
		if i := strings.LastIndexByte(raw, '-'); i >= 0 { // strip the -GOMAXPROCS suffix
			raw = raw[:i]
		}
		var ai, gf float64
		for i := 1; i < len(f); i++ {
			switch f[i] {
			case "AI(flop/byte)":
				ai, _ = strconv.ParseFloat(f[i-1], 64)
			case "GFLOP/s":
				gf, _ = strconv.ParseFloat(f[i-1], 64)
			}
		}
		if ai <= 0 || gf <= 0 {
			continue
		}
		pts = append(pts, point{label(raw), ai, gf})
	}
	return pts
}

func main() {
	peak := flag.Float64("peak", 25.51, "compute ceiling in GFLOP/s (from make roofline-ceiling)")
	bw := flag.Float64("bw", 18.39, "memory read bandwidth in GB/s (from make roofline-ceiling)")
	tpeak := flag.Float64("tpeak", 0, "optional theoretical compute peak (e.g. AVX2 ~100); 0 = hide")
	flag.Parse()

	pts := parseBench(bufio.NewScanner(os.Stdin))
	if len(pts) == 0 {
		fmt.Fprintln(os.Stderr, "roofline-plot: no points with AI(flop/byte)+GFLOP/s on stdin")
		os.Exit(1)
	}

	ridge := *peak / *bw // AI where memory roof meets compute roof

	// --- axis ranges (log10), padded to include every point and the ceilings.
	aiLo, aiHi := 0.25, 64.0
	gfHi := *peak
	for _, p := range pts {
		aiLo = math.Min(aiLo, p.ai)
		aiHi = math.Max(aiHi, p.ai)
		gfHi = math.Max(gfHi, p.gf)
	}
	if *tpeak > 0 {
		gfHi = math.Max(gfHi, *tpeak)
	}
	aiLo, aiHi = aiLo*0.7, aiHi*1.5
	gfLo, gfHiP := 1.0, gfHi*1.5

	// --- plot geometry.
	const W, H = 960, 600
	const x0, y0, plotW, plotH = 80.0, 70.0, 820.0, 440.0 // plot box: x0..x0+plotW, y0..y0+plotH
	lx0, lx1 := math.Log10(aiLo), math.Log10(aiHi)
	ly0, ly1 := math.Log10(gfLo), math.Log10(gfHiP)
	px := func(ai float64) float64 { return x0 + (math.Log10(ai)-lx0)/(lx1-lx0)*plotW }
	py := func(gf float64) float64 { return y0 + plotH - (math.Log10(gf)-ly0)/(ly1-ly0)*plotH }
	// roof(ai) is the lower of the two ceilings at a given AI.
	roof := func(ai float64) float64 { return math.Min(ai**bw, *peak) }

	memFill, memStroke := "#dbeafe", "#60a5fa"
	cFill, cStroke := "#ffedd5", "#fb923c"

	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	p("<!doctype html><html lang=\"ja\"><meta charset=\"utf-8\">\n")
	p("<title>実測ルーフライン</title>\n")
	p("<style>body{margin:0;background:#fff;font-family:'Helvetica Neue',Helvetica,Arial,'Hiragino Sans','Hiragino Kaku Gothic ProN',sans-serif;color:#111827}" +
		".wrap{max-width:980px;margin:0 auto;padding:16px}" +
		"#tip{position:fixed;pointer-events:none;background:#111827;color:#fff;font-size:12px;padding:6px 9px;border-radius:6px;opacity:0;transition:opacity .08s;white-space:nowrap}" +
		"circle.pt{cursor:pointer}</style>\n")
	p("<div class=\"wrap\">\n")
	p("<svg id=\"rl\" xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %d %d\" width=\"100%%\">\n", W, H)
	p("<rect width=\"%d\" height=\"%d\" fill=\"#ffffff\"/>\n", W, H)
	p("<text x=\"%d\" y=\"32\" font-size=\"18\" font-weight=\"600\" text-anchor=\"middle\">実測ルーフライン: 測った点が上限に近づく</text>\n", W/2)
	p("<text x=\"%d\" y=\"52\" font-size=\"11.5\" fill=\"#6b7280\" text-anchor=\"middle\">演算ピーク %.1f GFLOP/s、メモリ帯域 %.1f GB/s、リッジ %.2f flop/byte(make roofline-ceiling の実測)。点にカーソルを当てると詳細が出ます。</text>\n", W/2, *peak, *bw, ridge)

	// --- grid + ticks
	for _, t := range []float64{0.25, 0.5, 1, 2, 4, 8, 16, 32, 64} {
		if t < aiLo || t > aiHi {
			continue
		}
		x := px(t)
		p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#eef2f7\"/>\n", x, y0, x, y0+plotH)
		lab := strconv.FormatFloat(t, 'g', -1, 64)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10.5\" fill=\"#6b7280\" text-anchor=\"middle\">%s</text>\n", x, y0+plotH+16, lab)
	}
	for _, t := range []float64{1, 2, 5, 10, 20, 50, 100, 200} {
		if t < gfLo || t > gfHiP {
			continue
		}
		y := py(t)
		p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#eef2f7\"/>\n", x0, y, x0+plotW, y)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10.5\" fill=\"#6b7280\" text-anchor=\"end\">%g</text>\n", x0-8, y+3.5, t)
	}
	// axis titles
	p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" fill=\"#374151\" text-anchor=\"middle\">算術強度 (flop/byte)、対数</text>\n", x0+plotW/2, y0+plotH+40)
	p("<text transform=\"translate(22,%.1f) rotate(-90)\" font-size=\"12\" fill=\"#374151\" text-anchor=\"middle\">性能 (GFLOP/s)、対数</text>\n", y0+plotH/2)

	// --- roofline: memory diagonal up to the ridge, then compute horizontal.
	rx := math.Max(aiLo, math.Min(ridge, aiHi))
	p("<polyline fill=\"none\" stroke=\"#9ca3af\" stroke-width=\"2.5\" points=\"%.1f,%.1f %.1f,%.1f %.1f,%.1f\"/>\n",
		px(aiLo), py(roof(aiLo)), px(rx), py(*peak), px(aiHi), py(*peak))
	// region tints (memory-bound left of ridge, compute-bound right)
	p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" fill=\"%s\" text-anchor=\"start\">メモリ律速(メモリ帯域 %.1f GB/s で決まる斜線)</text>\n", px(aiLo)+6, py(roof(aiLo))-8, memStroke, *bw)
	p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" fill=\"%s\" text-anchor=\"end\">演算律速(演算ピーク %.1f GFLOP/s で決まる水平線)</text>\n", px(aiHi)-6, py(*peak)-8, cStroke, *peak)
	// ridge marker
	p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#9ca3af\" stroke-dasharray=\"3 3\"/>\n", px(ridge), py(*peak), px(ridge), y0+plotH)
	p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10\" fill=\"#6b7280\" text-anchor=\"middle\">リッジ %.2f</text>\n", px(ridge), py(*peak)-4, ridge)

	// optional theoretical peak (shows the spill gap)
	if *tpeak > 0 {
		y := py(*tpeak)
		p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#cbd5e1\" stroke-width=\"1.5\" stroke-dasharray=\"6 4\"/>\n", x0, y, x0+plotW, y)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10.5\" fill=\"#94a3b8\" text-anchor=\"end\">AVX2 の理論ピーク %.0f(register spill で届かない)</text>\n", x0+plotW, y-4, *tpeak)
	}

	// --- points: a dropline to the roof shows how close to the ceiling it is.
	for _, pt := range pts {
		x, yPt := px(pt.ai), py(pt.gf)
		r := roof(pt.ai)
		frac := pt.gf / r * 100
		fill, stroke := cFill, cStroke
		if pt.ai < ridge {
			fill, stroke = memFill, memStroke
		}
		// dropline from point up to the roof at this AI
		p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1.2\" stroke-dasharray=\"2 3\"/>\n", x, yPt, x, py(r), stroke)
		p("<circle class=\"pt\" cx=\"%.1f\" cy=\"%.1f\" r=\"7\" fill=\"%s\" stroke=\"%s\" stroke-width=\"2\" "+
			"data-name=\"%s\" data-ai=\"%g\" data-gf=\"%.2f\" data-roof=\"%.2f\" data-frac=\"%.0f\"/>\n",
			x, yPt, fill, stroke, pt.name, pt.ai, pt.gf, r, frac)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" font-weight=\"600\" fill=\"#111827\" text-anchor=\"middle\">%s</text>\n", x, yPt-12, pt.name)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10\" fill=\"#6b7280\" text-anchor=\"middle\">%.1f GF・上限の %.0f%%</text>\n", x, yPt+20, pt.gf, frac)
	}

	p("</svg>\n")
	p("<div id=\"tip\"></div>\n")
	// minimal interactivity: hover tooltip (zero deps).
	p("<script>\n")
	p("var tip=document.getElementById('tip');\n")
	p("document.querySelectorAll('circle.pt').forEach(function(c){\n")
	p("  c.addEventListener('mousemove',function(e){\n")
	p("    var d=c.dataset;\n")
	p("    tip.innerHTML=d.name+'<br>算術強度 '+d.ai+' flop/byte<br>'+d.gf+' GFLOP/s<br>上限 '+d.roof+' GF の '+d.frac+'%%';\n")
	p("    tip.style.left=(e.clientX+14)+'px';tip.style.top=(e.clientY+14)+'px';tip.style.opacity=1;\n")
	p("  });\n")
	p("  c.addEventListener('mouseleave',function(){tip.style.opacity=0;});\n")
	p("});\n")
	p("</script>\n")
	p("</div></html>\n")

	os.Stdout.WriteString(b.String())
}
