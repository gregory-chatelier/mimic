package main

import (
	"os"

	"github.com/gregory-chatelier/mimic/pkg/replayer"
)

// This is a helper program for testing the replayer. It is not part of the main mimic binary.
func main() {
	if len(os.Args) < 2 {
		panic("voucher file not provided")
	}
	voucherFile := os.Args[1]

	// For simplicity in the test, we are not passing preserveTiming and speed.
	// A more advanced test could pass these as flags.
	if _, err := replayer.Replay(voucherFile, false, 1.0); err != nil {
		os.Exit(1) // Exit with a non-zero status on error
	}
}
