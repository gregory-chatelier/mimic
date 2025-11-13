package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

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

		// Check SHA256 checksum
		hasher := sha256.New()
		hasher.Write(verifiableData)
		calculatedChecksum := hex.EncodeToString(hasher.Sum(nil))

		if calculatedChecksum == v.Signature.ChecksumSHA256 {
			fmt.Println("✔ SHA256 checksum matches")
		} else {
			fmt.Fprintf(os.Stderr, "Error: SHA256 checksum mismatch! Expected %s, calculated %s.\n", v.Signature.ChecksumSHA256, calculatedChecksum)
			os.Exit(1)
		}

		// Check TTL
		if v.TTL > 0 {
			expirationTime := v.RecordedAt.Add(v.TTL)
			if time.Now().After(expirationTime) {
				fmt.Fprintf(os.Stderr, "Error: Voucher expired on %s (TTL: %s).\n", expirationTime.Format(time.RFC3339), v.TTL.String())
				os.Exit(1)
			}
		}
		fmt.Println("✔ Voucher not expired")
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&publicKeyPath, "public-key", "mimic.pub", "Path to the public key file for verification")
}
