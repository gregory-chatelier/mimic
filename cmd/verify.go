package cmd

import (
	"crypto/sha256"
	"encoding/hex"
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
		return 1, fmt.Errorf("%w. Use 'mimic keygen' to create one", err)
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

	// 1. Get the canonical representation of the voucher.
	canonical := crypto.GetCanonicalVoucher(v)
	canonical.Signature = voucher.Signature{} // Clear signature for verification.

	// 2. Marshal the canonical voucher to get the verifiable data.
	verifiableData, err := yaml.Marshal(canonical)
	if err != nil {
		return 1, fmt.Errorf("failed to marshal canonical voucher for verification: %w", err)
	}

	// 3. Verify the checksum first for a quick integrity check.
	hasher := sha256.New()
	hasher.Write(verifiableData)
	calculatedChecksum := hex.EncodeToString(hasher.Sum(nil))
	if calculatedChecksum != v.Signature.ChecksumSHA256 {
		return 1, fmt.Errorf("voucher checksum is invalid! This indicates tampering (expected %s, got %s)", v.Signature.ChecksumSHA256, calculatedChecksum)
	}

	// 4. Verify the signature.
	if !crypto.VerifySignature(pk, verifiableData, signatureBytes) {
		return 1, fmt.Errorf("voucher signature is invalid! This indicates tampering")
	}

	// 5. Verify SHA256Output for integrity of recorded stdout/stderr
	if v.Metadata.SHA256Output != "" {
		outputHasher := sha256.New()
		for _, chunk := range v.Stdout {
			decoded, err := voucher.DecodeChunkData(chunk.DataB64)
			if err != nil {
				return 1, fmt.Errorf("failed to decode stdout chunk data for SHA256Output verification: %w", err)
			}
			outputHasher.Write(decoded)
		}
		for _, chunk := range v.Stderr {
			decoded, err := voucher.DecodeChunkData(chunk.DataB64)
			if err != nil {
				return 1, fmt.Errorf("failed to decode stderr chunk data for SHA256Output verification: %w", err)
			}
			outputHasher.Write(decoded)
		}
		calculatedOutputHash := hex.EncodeToString(outputHasher.Sum(nil))
		if calculatedOutputHash != v.Metadata.SHA256Output {
			return 1, fmt.Errorf("recorded output SHA256 hash mismatch! This indicates tampering (expected %s, got %s)", v.Metadata.SHA256Output, calculatedOutputHash)
		}
	}

	fmt.Println("Voucher signature and output integrity are valid.")
	return 0, nil
}

func init() {
	verifyCmd.Flags().StringVarP(&verifyPublicKeyPath, "public-key", "p", "", "Path to the public key for verification")
	_ = verifyCmd.MarkFlagRequired("public-key")
}



