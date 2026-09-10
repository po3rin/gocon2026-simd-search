// Command roofline-figures は docs/images の静的なルーフライン図
// (roofline-concept、rl-stage0..4、rl-batch、roofline-plot)を再生成する。
// 軸・灰色の屋根・点のスタイル・「上限の何 %」の縦線を、対話版の
// `make roofline-plot` と同じ見た目に揃えるのが目的。
//
// 数値は docs/workshop/workshop.md に載せた Codespaces(AMD EPYC 7763、4 コア)の
// 実測値。再計測したら次で再生成する:
//
//	make roofline-figures            # SVG + PNG(要 rsvg-convert)
//	go run ./cmd/roofline-figures -peak 25.59 -bw 20.80 -out docs/images
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// pt はルーフライン上の点 1 つ。
type pt struct {
	name   string  // 点の上に出すラベル
	ai     float64 // arithmetic intensity (flop/byte)
	gf     float64 // achieved GFLOP/s
	note   string  // small text under the point ("" = auto "x GF・上限の y%")
	noNote bool    // 注記を出さない(薄い参考点用)
	ghost  bool    // previous stage: drawn gray, no dropline
	vague  bool    // 位置は目安(flop が定義できない): 破線で縦線なし
	side   string  // label side: "" (above), "below"
}

// fig は出力する図 1 枚。
type fig struct {
	file     string
	title    string
	subtitle string
	points   []pt
	notes    []string // 図の下の枠に出す行(概念図のみ)
	concept  bool     // 概念図: 目盛りの数字を消し、点の代わりに領域ラベルを出す
}

const (
	W, H                 = 960, 600
	x0, y0, plotW, plotH = 80.0, 70.0, 820.0, 440.0
	memFill, memStroke   = "#dbeafe", "#60a5fa"
	cFill, cStroke       = "#ffedd5", "#fb923c"
	ghostFill, ghostStr  = "#f3f4f6", "#cbd5e1"
	roofColor            = "#9ca3af"
)

func main() {
	peak := flag.Float64("peak", 25.59, "compute ceiling in GFLOP/s (make roofline-ceiling)")
	bw := flag.Float64("bw", 20.80, "memory read bandwidth in GB/s (make roofline-ceiling)")
	tpeak := flag.Float64("tpeak", 0, "theoretical compute peak line (0 = hide; docs の図は Makefile が 110 を渡す)")
	out := flag.String("out", "docs/images", "output directory")
	flag.Parse()

	ridge := *peak / *bw
	figs := figures(*peak, *bw, ridge)
	for _, f := range figs {
		svg := render(f, *peak, *bw, *tpeak)
		path := filepath.Join(*out, f.file+".svg")
		if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(path)
	}
}

// figures は出力する図と、それぞれに載せる実測点の一覧。
func figures(peak, bw, ridge float64) []fig {
	stage0 := pt{name: "Stage 0 スカラ全探索", ai: 0.5, gf: 2.15}
	stage1 := pt{name: "Stage 1 SIMD 全探索", ai: 0.5, gf: 9.7}
	batchSIMD := pt{name: "バッチ SIMD(B=32)", ai: 16, gf: 13.3}
	batchScalar := pt{name: "バッチ スカラ(B=32)", ai: 16, gf: 2.24, side: "below"}
	stage2Int8 := pt{name: "Stage 2 int8", ai: 2, gf: 18.3, note: "18.3 Gop/s・上限の 72%"}
	stage3Bin := pt{name: "Stage 3 1bit 量子化", ai: 16, gf: 80, note: "46x・flop が無いので位置は目安", vague: true}
	stage4Rerank := pt{name: "Stage 4 1bit + rerank", ai: 16, gf: 80, note: "43x・Recall 0.87・位置は Stage 3 と同じ", vague: true}
	ghost := func(p pt) pt { p.ghost = true; p.noNote = true; return p }

	return []fig{
		{file: "roofline-concept", concept: true,
			title:    "ルーフラインの形",
			subtitle: "横軸は算術強度 AI(1 バイト運ぶごとに何回計算するか)、縦軸は性能。どのコードもこの線より上には行けない"},
		{file: "rl-stage0",
			title:    "Stage 0: スカラ全探索",
			subtitle: "算術強度 0.5、2.15 GFLOP/s。メモリ帯域から決まる上限(約 10)にも届いていない",
			points:   []pt{stage0}},
		{file: "rl-stage1",
			title:    "Stage 1: SIMD 化",
			subtitle: "全探索はメモリ帯域の上限に達する(9.7 GF、上限の 93%)",
			points:   []pt{ghost(stage0), stage1}},
		{file: "rl-batch",
			title:    "クエリのバッチ化(B=32・付録)",
			subtitle: "算術強度 が 0.5 から 16 に動き、リッジを越えて演算律速側へ。exact のまま SIMD がスカラより 5.9x 速い",
			points:   []pt{ghost(stage1), batchSIMD, batchScalar}},
		{file: "rl-stage2",
			title:    "Stage 2: int8 量子化",
			subtitle: "算術強度 が 0.5 から 2 に動きリッジを越える。ただし演算ピークの下(int8 内積の速さで頭打ち)。Recall 0.948",
			points:   []pt{ghost(stage1), stage2Int8}},
		{file: "rl-stage3",
			title:    "Stage 3: 1bit 量子化",
			subtitle: "データが 1/32 になりキャッシュに乗る。DRAM 帯域の制約から外れるが Recall 0.18(近似)",
			points:   []pt{ghost(stage2Int8), stage3Bin}},
		{file: "rl-stage4",
			title:    "Stage 4: 1bit で絞って fp32 SIMD で rerank",
			subtitle: "速度の位置は Stage 3 と同じ。差は精度(Recall 0.18 から 0.87)",
			points:   []pt{stage4Rerank}},
		{file: "roofline-plot",
			title:    "実測ルーフライン全体像(Codespaces / AMD EPYC 7763)",
			subtitle: "演算ピーク " + ftoa(peak) + " GFLOP/s、メモリ帯域 " + ftoa(bw) + " GB/s、リッジ " + strconv.FormatFloat(ridge, 'f', 2, 64) + " flop/byte",
			points:   []pt{stage0, stage1, stage2Int8, stage3Bin},
			notes: []string{
				"Stage 0 から 1: 縦に上がりメモリ帯域の上限で止まる(算術強度 0.5 はリッジの左)",
				"Stage 1 から 2: 算術強度 を右に動かすとリッジを越え、SIMD が効く側に入る",
				"Stage 3: データを 1/32 にしてキャッシュに乗せる。flop が無いので点の位置は目安",
			}},
	}
}

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

func render(f fig, peak, bw, tpeak float64) string {
	ridge := peak / bw
	aiLo, aiHi := 0.25, 64.0
	gfLo, gfHi := 1.0, math.Max(150, math.Max(peak, tpeak)*1.3)
	lx0, lx1 := math.Log10(aiLo), math.Log10(aiHi)
	ly0, ly1 := math.Log10(gfLo), math.Log10(gfHi)
	px := func(ai float64) float64 { return x0 + (math.Log10(ai)-lx0)/(lx1-lx0)*plotW }
	py := func(gf float64) float64 { return y0 + plotH - (math.Log10(gf)-ly0)/(ly1-ly0)*plotH }
	roof := func(ai float64) float64 { return math.Min(ai*bw, peak) }

	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	esc := func(s string) string {
		return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
	}

	p("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %d %d\">\n", W, H)
	p("<style>text{font-family:'Helvetica Neue',Helvetica,Arial,'Hiragino Sans','Hiragino Kaku Gothic ProN',sans-serif;fill:#111827}</style>\n")
	p("<rect width=\"%d\" height=\"%d\" fill=\"#ffffff\"/>\n", W, H)
	p("<text x=\"%d\" y=\"32\" font-size=\"18\" font-weight=\"600\" text-anchor=\"middle\">%s</text>\n", W/2, esc(f.title))
	p("<text x=\"%d\" y=\"52\" font-size=\"11.5\" fill=\"#6b7280\" text-anchor=\"middle\">%s</text>\n", W/2, esc(f.subtitle))

	// region tints (concept figure only)
	if f.concept {
		p("<polygon points=\"%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f\" fill=\"%s\" opacity=\"0.5\"/>\n",
			px(ridge), py(peak), px(aiHi), py(peak), px(aiHi), y0+plotH, px(ridge), y0+plotH, cFill)
		p("<polygon points=\"%.1f,%.1f %.1f,%.1f %.1f,%.1f %.1f,%.1f\" fill=\"%s\" opacity=\"0.5\"/>\n",
			px(aiLo), py(roof(aiLo)), px(ridge), py(peak), px(ridge), y0+plotH, px(aiLo), y0+plotH, memFill)
	}

	// grid + ticks
	for _, t := range []float64{0.25, 0.5, 1, 2, 4, 8, 16, 32, 64} {
		x := px(t)
		p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#eef2f7\"/>\n", x, y0, x, y0+plotH)
		if !f.concept {
			p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10.5\" fill=\"#6b7280\" text-anchor=\"middle\">%s</text>\n", x, y0+plotH+16, strconv.FormatFloat(t, 'g', -1, 64))
		}
	}
	for _, t := range []float64{1, 2, 5, 10, 20, 50, 100} {
		y := py(t)
		p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#eef2f7\"/>\n", x0, y, x0+plotW, y)
		if !f.concept {
			p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10.5\" fill=\"#6b7280\" text-anchor=\"end\">%g</text>\n", x0-8, y+3.5, t)
		}
	}
	xlab, ylab := "算術強度 (flop/byte)、対数", "性能 (GFLOP/s)、対数"
	if f.concept {
		xlab, ylab = "算術強度 (flop/byte)。右ほど 1 バイトあたりの計算が多い", "性能 (GFLOP/s)。上ほど速い"
	}
	p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" fill=\"#374151\" text-anchor=\"middle\">%s</text>\n", x0+plotW/2, y0+plotH+40, xlab)
	p("<text transform=\"translate(22,%.1f) rotate(-90)\" font-size=\"12\" fill=\"#374151\" text-anchor=\"middle\">%s</text>\n", y0+plotH/2, ylab)

	// roof
	p("<polyline fill=\"none\" stroke=\"%s\" stroke-width=\"2.5\" points=\"%.1f,%.1f %.1f,%.1f %.1f,%.1f\"/>\n",
		roofColor, px(aiLo), py(roof(aiLo)), px(ridge), py(peak), px(aiHi), py(peak))
	if f.concept {
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" font-weight=\"600\" fill=\"%s\" text-anchor=\"start\">メモリ帯域で決まる斜線</text>\n", px(aiLo)+8, py(roof(aiLo))+20, memStroke)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" font-weight=\"600\" fill=\"%s\" text-anchor=\"end\">演算ピークで決まる水平線</text>\n", px(aiHi)-6, py(peak)-10, cStroke)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"14\" font-weight=\"600\" fill=\"%s\" text-anchor=\"middle\">メモリ律速</text>\n", px(0.6), py(2.6), "#1d4ed8")
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11.5\" fill=\"%s\" text-anchor=\"middle\">データを運ぶのが間に合わない</text>\n", px(0.6), py(2.6)+18, "#1d4ed8")
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11.5\" fill=\"%s\" text-anchor=\"middle\">点がここなら 算術強度 を上げる(データ表現を変える)</text>\n", px(0.6), py(2.6)+36, "#1d4ed8")
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"14\" font-weight=\"600\" fill=\"%s\" text-anchor=\"middle\">演算律速</text>\n", px(12), py(4), "#c2410c")
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11.5\" fill=\"%s\" text-anchor=\"middle\">計算が間に合わない</text>\n", px(12), py(4)+18, "#c2410c")
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11.5\" fill=\"%s\" text-anchor=\"middle\">点がここなら実装効率を上げる(SIMD など)</text>\n", px(12), py(4)+36, "#c2410c")
	} else {
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" fill=\"%s\" text-anchor=\"start\">メモリ律速(メモリ帯域 %.1f GB/s で決まる斜線)</text>\n", px(aiLo)+8, py(roof(aiLo))+18, memStroke, bw)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" fill=\"%s\" text-anchor=\"end\">演算律速(演算ピーク %.1f GFLOP/s で決まる水平線)</text>\n", px(aiHi)-6, py(peak)-8, cStroke, peak)
	}
	// ridge
	p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-dasharray=\"3 3\"/>\n", px(ridge), py(peak), px(ridge), y0+plotH, roofColor)
	p("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"4\" fill=\"#ffffff\" stroke=\"%s\" stroke-width=\"2\"/>\n", px(ridge), py(peak), roofColor)
	if f.concept {
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" fill=\"#6b7280\" text-anchor=\"middle\">リッジ(境目)</text>\n", px(ridge), py(peak)-10)
	} else {
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10\" fill=\"#6b7280\" text-anchor=\"middle\">リッジ %.2f</text>\n", px(ridge), py(peak)-8, ridge)
	}
	// theoretical peak
	if tpeak > 0 && !f.concept {
		y := py(tpeak)
		p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#cbd5e1\" stroke-width=\"1.5\" stroke-dasharray=\"6 4\"/>\n", x0, y, x0+plotW, y)
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10.5\" fill=\"#94a3b8\" text-anchor=\"start\">AVX2 の理論ピーク %.0f(register spill で届かない)</text>\n", x0+8, y-4, tpeak)
	}

	// points
	for _, q := range f.points {
		x, y := px(q.ai), py(q.gf)
		fill, stroke := cFill, cStroke
		if q.ai < ridge {
			fill, stroke = memFill, memStroke
		}
		labFill := "#111827"
		if q.ghost {
			fill, stroke, labFill = ghostFill, ghostStr, "#9ca3af"
		}
		r := roof(q.ai)
		if !q.ghost && !q.vague {
			p("<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"1.2\" stroke-dasharray=\"2 3\"/>\n", x, y, x, py(r), stroke)
		}
		dash := ""
		if q.vague && !q.ghost {
			dash = " stroke-dasharray=\"3 2\""
		}
		p("<circle cx=\"%.1f\" cy=\"%.1f\" r=\"7\" fill=\"%s\" stroke=\"%s\" stroke-width=\"2\"%s/>\n", x, y, fill, stroke, dash)
		note := ""
		if !q.noNote {
			note = q.note
			if note == "" {
				note = fmt.Sprintf("%.1f GF・上限の %.0f%%", q.gf, q.gf/r*100)
			}
		}
		anchor, tx, ty1, ty2 := "middle", x, y-12, y+20
		if q.side == "below" {
			ty1, ty2 = y+22, y+36
		}
		p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" font-weight=\"600\" fill=\"%s\" text-anchor=\"%s\">%s</text>\n", tx, ty1, labFill, anchor, esc(q.name))
		if note != "" {
			p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10\" fill=\"#6b7280\" text-anchor=\"%s\">%s</text>\n", tx, ty2, anchor, esc(note))
		}
	}

	// notes box
	if len(f.notes) > 0 {
		bx, by := x0+plotW-420.0, y0+plotH-18-float64(len(f.notes))*16
		p("<rect x=\"%.1f\" y=\"%.1f\" width=\"420\" height=\"%.1f\" rx=\"6\" fill=\"#ffffff\" stroke=\"#e5e7eb\" opacity=\"0.95\"/>\n", bx, by-14, float64(len(f.notes))*16+18)
		for i, n := range f.notes {
			p("<text x=\"%.1f\" y=\"%.1f\" font-size=\"10.5\" fill=\"#374151\">%s</text>\n", bx+10, by+float64(i)*16, esc(n))
		}
	}
	p("</svg>\n")
	return b.String()
}
