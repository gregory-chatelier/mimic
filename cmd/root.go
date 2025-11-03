package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mimic",
	Short: "mimic - Record once. Replay forever.",
	Long: `mimic is a deterministic, tamper-proof command behavior recorder for developers and CI systems.
It records the behavioral contract of a shell command and stores it as a cryptographically verifiable voucher (.vcr file).
Later, that voucher can be replayed to reproduce the command's behavior exactly, without executing the original command again.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior if no subcommand is given
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Add global flags here if needed
	rootCmd.AddCommand(keygenCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(refreshCmd)
}
