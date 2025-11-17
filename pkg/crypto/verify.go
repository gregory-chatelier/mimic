package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
)

// VerifyVoucherIntegrity checks a voucher's TTL, checksum, signature, and output hash.
// It returns an error if any verification step fails.
func VerifyVoucherIntegrity(v *voucher.Voucher, publicKeyPath string) error {
	// 1. Check TTL
	if v.TTL > 0 && time.Since(v.RecordedAt) > v.TTL {
		return fmt.Errorf("voucher has expired (Recorded at %s, TTL %s)", v.RecordedAt.Format(time.RFC3339), v.TTL.String())
	}

	// 2. Load public key
	pk, err := LoadPublicKey(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load public key: %w", err)
	}

	// 3. Get verifiable data
	canonical := GetCanonicalVoucher(*v)
	canonical.Signature = voucher.Signature{}
	verifiableData, err := yaml.Marshal(canonical)
	if err != nil {
		return fmt.Errorf("failed to marshal canonical voucher for verification: %w", err)
	}

	// 4. Verify Checksum
	if err := verifyChecksum(v, verifiableData); err != nil {
		return err
	}

	// 5. Verify Signature
	if err := verifySignature(v, pk, verifiableData); err != nil {
		return err
	}

	// 6. Verify Output Hash
	if err := verifyOutputHash(v); err != nil {
		return err
	}

	return nil
}

// verifyChecksum verifies the SHA256 checksum of the voucher data.
func verifyChecksum(v *voucher.Voucher, verifiableData []byte) error {
	hasher := sha256.New()
	hasher.Write(verifiableData)
	calculatedChecksum := hex.EncodeToString(hasher.Sum(nil))

	if calculatedChecksum != v.Signature.ChecksumSHA256 {
		return fmt.Errorf("voucher checksum is invalid! This indicates tampering (expected %s, got %s)", v.Signature.ChecksumSHA256, calculatedChecksum)
	}
	return nil
}

// verifySignature verifies the Ed25519 signature of the voucher.
func verifySignature(v *voucher.Voucher, pk ed25519.PublicKey, verifiableData []byte) error {
	if v.Signature.SignatureB64 == "" {
		return fmt.Errorf("voucher is not signed")
	}

	signatureBytes, err := DecodeBase64(v.Signature.SignatureB64)
	if err != nil {
		return fmt.Errorf("failed to decode signature for validation: %w", err)
	}

	if !VerifySignature(pk, verifiableData, signatureBytes) {
		return fmt.Errorf("voucher signature is invalid! This indicates tampering")
	}
	return nil
}

// verifyOutputHash verifies the SHA256 hash of the combined stdout and stderr.
func verifyOutputHash(v *voucher.Voucher) error {
	if v.Metadata.SHA256Output == "" {
		return nil // Not all vouchers have this, so we skip if it's missing.
	}

	outputHasher := sha256.New()
	for _, chunk := range v.Stdout {
		decoded, err := voucher.DecodeChunkData(chunk.DataB64)
		if err != nil {
			return fmt.Errorf("failed to decode stdout chunk for hash verification: %w", err)
		}
		outputHasher.Write(decoded)
	}
	for _, chunk := range v.Stderr {
		decoded, err := voucher.DecodeChunkData(chunk.DataB64)
		if err != nil {
			return fmt.Errorf("failed to decode stderr chunk for hash verification: %w", err)
		}
		outputHasher.Write(decoded)
	}

	calculatedOutputHash := hex.EncodeToString(outputHasher.Sum(nil))
	if calculatedOutputHash != v.Metadata.SHA256Output {
		return fmt.Errorf("recorded output SHA256 hash mismatch! This indicates tampering (expected %s, got %s)", v.Metadata.SHA256Output, calculatedOutputHash)
	}
	return nil
}
