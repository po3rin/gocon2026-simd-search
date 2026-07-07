// Command isa-report checks which CPU features this machine exposes for the
// archsimd APIs used in this repo. Feature requirements follow pkg.go.dev:
// https://pkg.go.dev/simd/archsimd
//
// Run (amd64 + GOEXPERIMENT=simd):
//
//	GOARCH=amd64 GOEXPERIMENT=simd go run ./cmd/isa-report/
//
// Or: make isa-report
package main

func main() { run() }
