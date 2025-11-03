package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

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
		voucherFile := args[0]

		// Input Validation
		// 1. Validate voucher file existence
		if _, err := os.Stat(voucherFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Voucher file not found at %s\n", voucherFile)
			os.Exit(1)
		}

		// Read voucher file
		data, err := os.ReadFile(voucherFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read voucher file %s: %v\n", voucherFile, err)
			os.Exit(1)
		}

		var v voucher.Voucher
		if err := yaml.Unmarshal(data, &v); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unmarshal voucher from %s: %v\n", voucherFile, err)
			os.Exit(1)
		}

		fmt.Printf("Command: %s\n", strings.Join(v.Command.Argv, " "))
		fmt.Printf("Recorded: %s\n", v.RecordedAt.Format(time.RFC3339))
		fmt.Printf("Exit Code: %d\n", v.ExitCode)
		fmt.Printf("Duration: %dms\n", v.DurationMs)

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
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}
