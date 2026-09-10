package main

import (
	"bufio"
	"strings"
	"testing"
)

// go test -bench の実出力の形でパースを固定する(メトリクス名や列が変わったら
// ここで気付く)。AI+GFLOP/s を報告しないベンチ(SearchBinary)は点にならない。
func TestParseBench(t *testing.T) {
	in := `goos: linux
BenchmarkSearchNaive-4     33  35856977 ns/op  0.5 AI(flop/byte)  2.142 GFLOP/s  153.6 MB/query
BenchmarkSearchSIMD-4     150   7900000 ns/op  0.5 AI(flop/byte)  9.7 GFLOP/s  153.6 MB/query
BenchmarkSearchInt8-4     280   4200000 ns/op  2 AI(flop/byte)  18.3 Gop/s  38.4 MB/query
BenchmarkSearchBinary-4  2500    770000 ns/op  4.8 MB/query
PASS`
	pts := parseBench(bufio.NewScanner(strings.NewReader(in)))
	if len(pts) != 3 {
		t.Fatalf("got %d points, want 3: %+v", len(pts), pts)
	}
	if pts[0].name != "Stage 0 スカラ全探索" || pts[0].ai != 0.5 || pts[0].gf != 2.142 {
		t.Errorf("pts[0] = %+v", pts[0])
	}
	if pts[1].name != "Stage 1 SIMD 全探索" {
		t.Errorf("pts[1] = %+v", pts[1])
	}
	if pts[2].name != "Stage 2 int8" || pts[2].ai != 2 || pts[2].gf != 18.3 {
		t.Errorf("pts[2] = %+v", pts[2])
	}
}
