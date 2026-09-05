//go:build !(goexperiment.simd && (amd64 || arm64))

package main

import (
	"fmt"
	"os"
)

// run は GOEXPERIMENT=simd (amd64 / arm64) 以外のビルドでは案内だけ出して終了する。
// これで GOEXPERIMENT 未指定や wasm 等の `go test ./...` / `go build ./...` を壊さない。
func run() {
	fmt.Fprintln(os.Stderr, "isa-report requires GOEXPERIMENT=simd on amd64 or arm64.")
	fmt.Fprintln(os.Stderr, "run: make isa-report   (= GOEXPERIMENT=simd go run ./cmd/isa-report/)")
	os.Exit(1)
}
