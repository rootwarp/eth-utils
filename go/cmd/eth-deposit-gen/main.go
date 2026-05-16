package main

import (
	"fmt"
	"io"
	"os"
)

const version = "eth-deposit-gen v0.0.0"

// run writes the version string to w and returns exit code 0.
// It is separated from main to allow testing without process exit.
func run(w io.Writer) int {
	fmt.Fprintln(w, version)
	return 0
}

func main() {
	os.Exit(run(os.Stdout))
}
