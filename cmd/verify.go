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
	if err := validation.ValidateFileExists(voucherFile, "Voucher file"); err != nil {
		return 1, err
	}
	if err := validation.ValidateFileExists(verifyPublicKeyPath, "Public key file"); err != nil {
		return 1, fmt.Errorf("%w. Use 'mimic keygen' to create one", err)
	}

	// Read and unmarshal voucher
	data, err := os.ReadFile(voucherFile)
	if err != nil {
		return 1, fmt.Errorf("failed to read voucher file %s: %w", voucherFile, err)
	}
	var v voucher.Voucher
	if err := yaml.Unmarshal(data, &v); err != nil {
		return 1, fmt.Errorf("failed to unmarshal voucher from %s: %w", voucherFile, err)
	}

	// Centralized verification
	if err := crypto.VerifyVoucherIntegrity(&v, verifyPublicKeyPath); err != nil {
		return 1, fmt.Errorf("voucher verification failed: %w", err)
	}

	fmt.Println("Voucher signature and output integrity are valid.")
	return 0, nil
}

func init() {
	verifyCmd.Flags().StringVarP(&verifyPublicKeyPath, "public-key", "p", "", "Path to the public key for verification")
	_ = verifyCmd.MarkFlagRequired("public-key")
}
