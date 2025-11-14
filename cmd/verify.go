package cmd

import (
	"fmt"
	"os"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/validation"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	verifyPublicKeyPath string
)

var verifyCmd = &cobra.Command{
	Use:   "verify <voucher>",
	Short: "Verify the cryptographic signature of a voucher",
	Long: `The verify command checks the integrity and authenticity of a .vcr voucher file
by verifying its Ed25519 signature against a provided public key.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if exitCode, err := runVerifyCmd(cmd, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(exitCode)
		}
	},
}

func runVerifyCmd(cmd *cobra.Command, args []string) (int, error) {
	voucherFile := args[0]

	// Input Validation
	// 1. Validate voucher file existence
	if err := validation.ValidateFileExists(voucherFile, "Voucher file"); err != nil {
		return 1, err
	}
	// 2. Validate public key file existence
	if err := validation.ValidateFileExists(verifyPublicKeyPath, "Public key file"); err != nil {
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

	if v.Signature.SignatureB64 == "" {
		return 1, fmt.Errorf("voucher is not signed")
	}

	// Load public key
	pk, err := crypto.LoadPublicKey(verifyPublicKeyPath)
	if err != nil {
		return 1, fmt.Errorf("loading public key: %w", err)
	}

	// Decode signature
	signatureBytes, err := crypto.DecodeBase64(v.Signature.SignatureB64)
	if err != nil {
		return 1, fmt.Errorf("decoding signature: %w", err)
	}

	// Prepare data for verification (voucher without signature)
	v.Signature = voucher.Signature{}
	verifiableData, err := yaml.Marshal(v)
	if err != nil {
		return 1, fmt.Errorf("failed to marshal voucher for verification: %w", err)
	}

	// Verify signature
	if crypto.VerifySignature(pk, verifiableData, signatureBytes) {
		fmt.Println("Voucher signature is valid.")
		return 0, nil
	} else {
		return 1, fmt.Errorf("voucher signature is invalid! This indicates tampering")
	}
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&verifyPublicKeyPath, "public-key", "mimic.pub", "Path to the public key file for verification")
}
