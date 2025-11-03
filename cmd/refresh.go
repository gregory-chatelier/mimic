package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh <voucher>",
	Short: "Re-run command to refresh expired or invalid voucher",
	Long: `The refresh command reads an existing .vcr voucher file, extracts the original command,
re-runs it, and creates a new voucher, overwriting the old one. 
It can be used to update vouchers that have expired or whose command behavior has changed.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		voucherFile := args[0]

		// Input Validation
		// 1. Validate voucher file existence
		if _, err := os.Stat(voucherFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Voucher file not found at %s\n", voucherFile)
			os.Exit(1)
		}

		// Read existing voucher
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

		// Get command to re-run
		cmdToRecord := v.Command.Argv

		fmt.Printf("Refreshing voucher by re-running command: %s\n", strings.Join(cmdToRecord, " "))

		// Record the command again, overwriting the old voucher
		// We re-use the settings from the old voucher where possible (e.g., withEnv, preserveTiming)
		// For simplicity in this implementation, we will use the default settings for the new recording.
		// A more advanced implementation could parse all settings from the old voucher.

		_, err = recorder.Record(cmdToRecord, voucherFile, len(v.Command.Env) > 0, v.TTL, len(v.Stdout) > 1 || len(v.Stderr) > 1, voucherFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error refreshing voucher: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Voucher %s has been refreshed.\n", voucherFile)
	},
}

func init() {
	rootCmd.AddCommand(refreshCmd)
}
