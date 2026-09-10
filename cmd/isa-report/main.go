// Command isa-report checks which CPU features this machine exposes for the
// archsimd APIs used in this repo. Feature requirements follow pkg.go.dev:
// https://pkg.go.dev/simd/archsimd
//
// Run:
//
//	make isa-report          # 実行中のアーキの一覧(arm64 なら Neon 版)
//	make isa-report-amd64    # Mac から amd64 側の一覧(Rosetta 実行・FMA=false)
//	GOARCH=amd64 GOEXPERIMENT=simd go run ./cmd/isa-report/
package main

func main() { run() }
