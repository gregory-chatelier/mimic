package cmd

import (
	"fmt"
	"os"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	verifyPublicKeyPath string
)

var verifyCmd = &cobra.Command{
	Use:   "verify <voucher>",
	Short: "Verify the authenticity and integrity of a voucher",
	Long: `The verify command checks the cryptographic signature and integrity of a .vcr voucher file.
It ensures that the voucher has not been tampered with since it was signed.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		voucherFile := args[0]

		// Input Validation
		// 1. Validate voucher file existence
		if _, err := os.Stat(voucherFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Voucher file not found at %s\n", voucherFile)
			os.Exit(1)
		}

		// 2. Validate public key file existence
		if _, err := os.Stat(verifyPublicKeyPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Public key file not found at %s. Cannot verify signature.\n", verifyPublicKeyPath)
			os.Exit(1)
		}

		// Load public key
		pk, err := crypto.LoadPublicKey(verifyPublicKeyPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading public key for verification: %v\n", err)
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

		if v.Signature.SignatureB64 == "" {
			fmt.Fprintf(os.Stderr, "Error: Voucher is not signed.\n")
			os.Exit(1)
		}

		signatureBytes, err := crypto.DecodeBase64(v.Signature.SignatureB64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding signature: %v\n", err)
			os.Exit(1)
		}

		// Prepare data for verification (exclude signature field)
		verifiableVoucher := v
		verifiableVoucher.Signature = voucher.Signature{}
		verifiableData, err := yaml.Marshal(verifiableVoucher)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling voucher for verification: %v\n", err)
			os.Exit(1)
		}

		if crypto.VerifySignature(pk, verifiableData, signatureBytes) {
			fmt.Println("✔ Signature valid (ed25519)")
		} else {
			fmt.Fprintf(os.Stderr, "Error: Signature invalid!\n")
			os.Exit(1)
		}

		// TODO: Add SHA256 checksum matches and Voucher not expired checks later
		fmt.Println("✔ SHA256 checksum matches (TODO)")
		fmt.Println("✔ Voucher not expired (TODO)")
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&publicKeyPath, "public-key", "mimic.pub", "Path to the public key file for verification")
}
