//go:build !(goexperiment.simd && amd64)

package main

import (
	"fmt"
	"os"
)

// run は amd64 + GOEXPERIMENT=simd 以外のビルドでは案内だけ出して終了する。
// これで arm64 (Apple Silicon) の `go test ./...` / `go build ./...` を壊さない。
func run() {
	fmt.Fprintln(os.Stderr, "isa-report requires amd64 + GOEXPERIMENT=simd.")
	fmt.Fprintln(os.Stderr, "run: make isa-report   (= GOARCH=amd64 GOEXPERIMENT=simd go run ./cmd/isa-report/)")
	os.Exit(1)
}
