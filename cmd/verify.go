package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

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
	Short: "Verify the authenticity and integrity of a voucher",
	Long: `The verify command checks the cryptographic signature and integrity of a .vcr voucher file.
It ensures that the voucher has not been tampered with since it was signed.`,
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
		return 1, fmt.Errorf("%w. Cannot verify signature", err)
	}

	// Load public key
	pk, err := crypto.LoadPublicKey(verifyPublicKeyPath)
	if err != nil {
		return 1, fmt.Errorf("loading public key for verification: %w", err)
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

	signatureBytes, err := crypto.DecodeBase64(v.Signature.SignatureB64)
	if err != nil {
		return 1, fmt.Errorf("decoding signature: %w", err)
	}

	// Prepare data for verification (exclude signature field)
	verifiableVoucher := v
	verifiableVoucher.Signature = voucher.Signature{}
	verifiableData, err := yaml.Marshal(verifiableVoucher)
	if err != nil {
		return 1, fmt.Errorf("marshalling voucher for verification: %w", err)
	}

	if !crypto.VerifySignature(pk, verifiableData, signatureBytes) {
		return 1, fmt.Errorf("signature invalid")
	}
	fmt.Println("✔ Signature valid (ed25519)")

	// Check SHA256 checksum
	hasher := sha256.New()
	hasher.Write(verifiableData)
	calculatedChecksum := hex.EncodeToString(hasher.Sum(nil))

	if calculatedChecksum != v.Signature.ChecksumSHA256 {
		return 1, fmt.Errorf("SHA256 checksum mismatch! Expected %s, calculated %s", v.Signature.ChecksumSHA256, calculatedChecksum)
	}
	fmt.Println("✔ SHA256 checksum matches")

	// Check TTL
	if v.TTL > 0 {
		expirationTime := v.RecordedAt.Add(v.TTL)
		if time.Now().After(expirationTime) {
			return 1, fmt.Errorf("voucher expired on %s (TTL: %s)", expirationTime.Format(time.RFC3339), v.TTL.String())
		}
	}
	fmt.Println("✔ Voucher not expired")

	return 0, nil
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVar(&verifyPublicKeyPath, "public-key", "mimic.pub", "Path to the public key file for verification")
}
