package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time using -ldflags "-X 'github.com/gregory-chatelier/mimic/cmd.version=vX.Y.Z'"
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "mimic",
	Short: "mimic - Record once. Replay forever.",
	Long: `mimic is a deterministic, tamper-proof command behavior recorder
It records the behavior of a shell command : its standard output, standard error, exit code, and environment metadata, and stores it as a cryptographically verifiable voucher (.vcr file).
Later, that voucher can be replayed to reproduce the command’s behavior exactly, without executing the original command again.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Default behavior if no subcommand is given
		if err := cmd.Help(); err != nil {
			fmt.Fprintf(os.Stderr, "Error displaying help: %v\n", err)
			os.Exit(1)
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of mimic",
	Long:  `All software has versions. This is mimic's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
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
	rootCmd.AddCommand(replayCmd)
	rootCmd.AddCommand(recordCmd)
	rootCmd.AddCommand(versionCmd) // Add the version command

	rootCmd.SetHelpCommand(nil) // Disable the default 'help' command

	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
