//go:build !js || !wasm

package main

// main exists only so `go build ./...` succeeds on the host toolchain:
// api.go and its tests must compile and run on darwin/linux (that is the
// whole point of the tagged/untagged split), and a package main without a
// main function does not build. The real entry point is in main.go, under
// `//go:build js && wasm`.
func main() {}
