// Command isa-report は、このリポジトリで使う archsimd API に必要な CPU 機能を
// このマシンがどこまで持っているかを一覧にする。必要機能の対応は pkg.go.dev に従う:
// https://pkg.go.dev/simd/archsimd
//
// 実行方法:
//
//	make isa-report          # 実行中のアーキの一覧(arm64 なら Neon 版)
//	make isa-report-amd64    # Mac から amd64 側の一覧(Rosetta 実行・FMA=false)
//	GOARCH=amd64 GOEXPERIMENT=simd go run ./cmd/isa-report/
package main

func main() { run() }
