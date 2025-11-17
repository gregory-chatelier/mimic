package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/validation"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <voucher>",
	Short: "Show voucher metadata and summary",
	Long:  `The inspect command reads a .vcr voucher file and prints its metadata and summary in a human-readable format.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if exitCode, err := runInspectCmd(cmd, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(exitCode)
		}
	},
}

func runInspectCmd(cmd *cobra.Command, args []string) (int, error) {
	voucherFile := args[0]

	// Input Validation
	// 1. Validate voucher file existence
	if err := validation.ValidateFileExists(voucherFile, "Voucher file"); err != nil {
		return 1, err
	}

	// Read voucher file
	data, err := os.ReadFile(voucherFile)
	if err != nil {
		return 1, fmt.Errorf("failed to read voucher file %s: %w", voucherFile, err)
	}

	var v voucher.Voucher
	if err := yaml.Unmarshal(data, &v); err != nil {
		return 1, fmt.Errorf("failed to unmarshal voucher from %s: %w", voucherFile, err)
	}

	if v.Command.Raw != "" {
		fmt.Printf("Command: %s\n", v.Command.Raw)
	} else {
		fmt.Printf("Command: %s\n", strings.Join(v.Command.Argv, " "))
	}
	fmt.Printf("Recorded: %s\n", v.RecordedAt.Format(time.RFC3339))
	fmt.Printf("Exit Code: %d\n", v.ExitCode)
	// Display duration with more precision
	if v.DurationNs > 0 {
		fmt.Printf("Duration: %s\n", time.Duration(v.DurationNs).String())
	} else {
		fmt.Printf("Duration: 0s\n") // Or "<1ns" if we want to be super precise for 0
	}

	if len(v.Command.Env) > 0 {
		fmt.Println("Environment:")
		for k, val := range v.Command.Env {
			fmt.Printf("  %s=%s\n", k, val)
		}
	}

	if v.Signature.SignatureB64 != "" {
		fmt.Printf("Signature: %s (algorithm: %s, key: %s)\n", "present", v.Signature.Algorithm, v.Signature.KeyID)
		// Attempt to verify if public key is available
		// This part is for future implementation or if a default public key is configured
	} else {
		fmt.Println("Signature: none")
	}

	if v.TTL > 0 {
		fmt.Printf("TTL: %s\n", v.TTL.String())
		if time.Since(v.RecordedAt) > v.TTL {
			fmt.Println("Status: Expired")
		} else {
			fmt.Printf("Status: Valid for %s\n", v.TTL-time.Since(v.RecordedAt))
		}
	}
	return 0, nil
}
