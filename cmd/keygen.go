package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/spf13/cobra"
)

var (
	keyOutputPath string
)

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a signing key pair for secure vouchers",
	Long: `The keygen command generates a new Ed25519 private/public key pair.
The private key is used to sign vouchers, and the public key is used to verify them.
By default, keys are saved as 'mimic.key' and 'mimic.pub' in the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		if exitCode, err := runKeygenCmd(cmd, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(exitCode)
		}
	},
}

func runKeygenCmd(cmd *cobra.Command, args []string) (int, error) {
	if keyOutputPath == "" {
		keyOutputPath = "."
	}

	privateKeyPath := filepath.Join(keyOutputPath, "mimic.key")
	publicKeyPath := filepath.Join(keyOutputPath, "mimic.pub")

	err := crypto.GenerateKeyPair(privateKeyPath, publicKeyPath)
	if err != nil {
		return 1, fmt.Errorf("generating key pair: %w", err)
	}

	fmt.Printf("Successfully generated key pair:\n  Private Key: %s\n  Public Key: %s\n", privateKeyPath, publicKeyPath)
	return 0, nil
}


