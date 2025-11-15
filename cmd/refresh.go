package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/gregory-chatelier/mimic/pkg/validation"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	refreshPublicKeyPath string
)

var refreshCmd = &cobra.Command{
	Use:   "refresh <voucher>",
	Short: "Re-run command to refresh expired or invalid voucher",
	Long: `The refresh command reads an existing .vcr voucher file, extracts the original command,
re-runs it, and creates a new voucher, overwriting the old one. 
It can be used to update vouchers that have expired or whose command behavior has changed.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if exitCode, err := runRefreshCmd(cmd, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(exitCode)
		}
	},
}

func runRefreshCmd(cmd *cobra.Command, args []string) (int, error) {
	voucherFile := args[0]

	// Input Validation
	// 1. Validate voucher file existence
	if err := validation.ValidateFileExists(voucherFile, "Voucher file"); err != nil {
		return 1, err
	}

	// Read existing voucher
	data, err := os.ReadFile(voucherFile)
	if err != nil {
		return 1, fmt.Errorf("failed to read voucher file %s: %w", voucherFile, err)
	}

	var v voucher.Voucher
	if err := yaml.Unmarshal(data, &v); err != nil {
		return 1, fmt.Errorf("failed to unmarshal voucher from %s: %w", voucherFile, err)
	}

	// Verify signature of original voucher if present
	if v.Signature.SignatureB64 != "" {
		if refreshPublicKeyPath == "" {
			return 1, fmt.Errorf("original voucher is signed, but --public-key was not provided for verification")
		}
		if err := validation.ValidateFileExists(refreshPublicKeyPath, "Public key file"); err != nil {
			return 1, fmt.Errorf("failed to verify original voucher signature: %w", err)
		}

		pk, err := crypto.LoadPublicKey(refreshPublicKeyPath)
		if err != nil {
			return 1, fmt.Errorf("failed to load public key for verification: %w", err)
		}

		signatureBytes, err := crypto.DecodeBase64(v.Signature.SignatureB64)
		if err != nil {
			return 1, fmt.Errorf("failed to decode signature for verification: %w", err)
		}

		verifiableVoucher := v
		verifiableVoucher.Signature = voucher.Signature{}
		verifiableData, err := yaml.Marshal(verifiableVoucher)
		if err != nil {
			return 1, fmt.Errorf("failed to marshal voucher for verification: %w", err)
		}

		if !crypto.VerifySignature(pk, verifiableData, signatureBytes) {
			return 1, fmt.Errorf("original voucher signature is invalid! This indicates tampering")
		}
		fmt.Fprintln(os.Stderr, "Warning: Original voucher is signed and verified. Refreshed voucher will need re-signing.")
	}

	// Get command to re-run
	cmdToRecord := v.Command.Argv

	fmt.Printf("Refreshing voucher by re-running command: %s\n", strings.Join(cmdToRecord, " "))

	// Record the command again, overwriting the old voucher
	// We re-use the settings from the old voucher where possible (e.g., withEnv, preserveTiming)
	// For simplicity in this implementation, we will use the default settings for the new recording.
	// A more advanced implementation could parse all settings from the old voucher.

	var envVarsToCapture []string
	if len(v.Command.Env) > 0 {
		envVarsToCapture = []string{} // Capture all env vars if original had them
	}

	_, err = recorder.Record(cmdToRecord, voucherFile, envVarsToCapture, v.TTL, v.PreserveTiming, []string{})
	if err != nil {
		return 1, fmt.Errorf("refreshing voucher: %w", err)
	}

	fmt.Printf("Voucher %s has been refreshed.\n", voucherFile)
	return 0, nil
}

func init() {
	refreshCmd.Flags().StringVarP(&refreshPublicKeyPath, "public-key", "p", "", "Path to the public key for verifying the original voucher (if signed)")
}



