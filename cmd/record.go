package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/spf13/cobra"
)

var (
	outputFile      string
	signVoucher     bool
	privateKeyPath  string
	withEnv         bool
	ttl             string
	preserveTiming  bool
	prevVoucherPath string
	redactPatterns  []string
)

var recordCmd = &cobra.Command{
	Use:   "record [flags] -- <command> [args...]",
	Short: "Record a command's behavior and save it to a .vcr file",
	Long: `The record command executes a given shell command, captures its standard output, standard error, exit code, and environment metadata,
and stores it as a cryptographically verifiable voucher (.vcr file).`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if exitCode, err := runRecordCmd(cmd, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(exitCode)
		}
	},
}

func runRecordCmd(cmd *cobra.Command, args []string) (int, error) {
	// Find the position of '--' to separate mimic flags from the command to be recorded
	separatorIdx := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIdx = i
			break
		}
	}

	var cmdToRecord []string
	if separatorIdx != -1 {
		cmdToRecord = args[separatorIdx+1:]
	} else {
		// If no '--' is found, assume all args are part of the command to record
		cmdToRecord = args
	}

	if len(cmdToRecord) == 0 {
		_ = cmd.Help()
		return 1, fmt.Errorf("no command provided to record")
	}

	// Validate that the command exists in the PATH
	if _, err := exec.LookPath(cmdToRecord[0]); err != nil {
		return 1, fmt.Errorf("command '%s' not found in PATH", cmdToRecord[0])
	}

	// Input Validation
	if signVoucher {
		if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
			return 1, fmt.Errorf("private key file not found at %s. Use 'mimic keygen' to create one", privateKeyPath)
		}
	}

	if prevVoucherPath != "" {
		if _, err := os.Stat(prevVoucherPath); os.IsNotExist(err) {
			return 1, fmt.Errorf("previous voucher file not found at %s", prevVoucherPath)
		}
	}

	if outputFile == "" {
		// Default output file name based on the command
		outputFile = strings.ReplaceAll(cmdToRecord[0], " ", "_") + ".vcr"
	}

	durationTTL := time.Duration(0)
	if ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return 1, fmt.Errorf("parsing TTL duration: %w", err)
		}
		durationTTL = d
	}

	var envVarsToCapture []string
	if withEnv {
		envVarsToCapture = []string{} // Empty slice means capture all
	}

	v, err := recorder.Record(cmdToRecord, outputFile, envVarsToCapture, durationTTL, preserveTiming, prevVoucherPath, redactPatterns)
	if err != nil {
		return 1, fmt.Errorf("recording command: %w", err)
	}

	if !signVoucher {
		fmt.Printf("Voucher recorded to %s\n", outputFile)
		return 0, nil
	}

	// Load private key
	pk, err := crypto.LoadPrivateKey(privateKeyPath)
	if err != nil {
		return 1, fmt.Errorf("loading private key for signing: %w", err)
	}

	// Sign the voucher and get the final YAML data
	data, err := crypto.SignVoucher(*v, pk)
	if err != nil {
		return 1, fmt.Errorf("signing voucher: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return 1, fmt.Errorf("failed to write signed voucher to file %s: %w", outputFile, err)
	}
	fmt.Printf("Voucher signed and recorded to %s\n", outputFile)

	return 0, nil
}

func init() {
	rootCmd.AddCommand(recordCmd)

	recordCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output voucher file (default: auto-named based on command)")
	recordCmd.Flags().BoolVar(&signVoucher, "sign", false, "Sign the voucher with default key")
	recordCmd.Flags().StringVar(&privateKeyPath, "private-key", "mimic.key", "Path to the private key file for signing")
	recordCmd.Flags().BoolVar(&withEnv, "with-env", false, "Include environment variables in the recording")
	recordCmd.Flags().StringVar(&ttl, "ttl", "", "Expire voucher after specified duration (e.g., 24h, 30m)")
	recordCmd.Flags().BoolVar(&preserveTiming, "preserve-timing", false, "Record time intervals between outputs")
	recordCmd.Flags().StringVar(&prevVoucherPath, "previous-voucher", "", "Path to the previous voucher to create a lineage")
	recordCmd.Flags().StringSliceVar(&redactPatterns, "redact", []string{}, "Environment variable names or regex patterns to redact (can be specified multiple times)")
	// recordCmd.Flags().StringVar(&note, "note", "", "Add an annotation or description")
	// recordCmd.gofmt Flags().BoolVar(&noStderr, "no-stderr", false, "Ignore stderr in recording")
}
