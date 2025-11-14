package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/gregory-chatelier/mimic/pkg/validation"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
)

// GenerateKeyPair generates a new Ed25519 private/public key pair and saves them to the specified paths.
func GenerateKeyPair(privateKeyPath, publicKeyPath string) error {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}

	// Save private key
	pemPrivate := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: privateKey,
	}
	if err := os.WriteFile(privateKeyPath, pem.EncodeToMemory(pemPrivate), 0600); err != nil {
		return fmt.Errorf("failed to write private key to %s: %w", privateKeyPath, err)
	}

	// Save public key
	pemPublic := &pem.Block{
		Type:  "ED25519 PUBLIC KEY",
		Bytes: publicKey,
	}
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(pemPublic), 0644); err != nil {
		return fmt.Errorf("failed to write public key to %s: %w", publicKeyPath, err)
	}

	return nil
}

// LoadPrivateKey loads an Ed25519 private key from a file.
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	// Validate file permissions before reading the key
	if err := validation.ValidatePrivateKeyPermissions(path); err != nil {
		return nil, err
	}

	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %s: %w", path, err)
	}

	pemBlock, remaining := pem.Decode(keyBytes)
	if pemBlock == nil {
		return nil, fmt.Errorf("invalid PEM format in private key file %s", path)
	}
	if len(remaining) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: Private key file %s contains %d extra bytes after PEM block.\n", path, len(remaining))
	}
	if pemBlock.Type != "ED25519 PRIVATE KEY" {
		return nil, fmt.Errorf("invalid PEM block type in private key file %s: expected 'ED25519 PRIVATE KEY', got '%s'", path, pemBlock.Type)
	}

	privateKey := ed25519.PrivateKey(pemBlock.Bytes)
	return privateKey, nil
}

// SignData signs the given data with the provided private key.
func SignData(privateKey ed25519.PrivateKey, data []byte) ([]byte, error) {
	signature := ed25519.Sign(privateKey, data)
	return signature, nil
}

// VerifySignature verifies the given data with the provided public key and signature.
func VerifySignature(publicKey ed25519.PublicKey, data, signature []byte) bool {
	return ed25519.Verify(publicKey, data, signature)
}

// LoadPublicKey loads an Ed25519 public key from a file.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file %s: %w", path, err)
	}

	pemBlock, remaining := pem.Decode(keyBytes)
	if pemBlock == nil {
		return nil, fmt.Errorf("invalid PEM format in public key file %s", path)
	}
	if len(remaining) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: Public key file %s contains %d extra bytes after PEM block.\n", path, len(remaining))
	}
	if pemBlock.Type != "ED25519 PUBLIC KEY" {
		return nil, fmt.Errorf("invalid PEM block type in public key file %s: expected 'ED25519 PUBLIC KEY', got '%s'", path, pemBlock.Type)
	}

	publicKey := ed25519.PublicKey(pemBlock.Bytes)
	return publicKey, nil
}

// EncodeBase64 encodes a byte slice to a base64 string.
func EncodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeBase64 decodes a base64 string to a byte slice.
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// SignVoucher prepares a voucher for signing, signs it, and returns the final YAML bytes.
func SignVoucher(v voucher.Voucher, privateKey ed25519.PrivateKey) ([]byte, error) {
	// 1. Prepare data for signing (clear signature field)
	signableVoucher := v
	signableVoucher.Signature = voucher.Signature{}
	
	// 2. Marshal the signable voucher to YAML
	verifiableData, err := yaml.Marshal(signableVoucher)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal voucher for signing: %w", err)
	}

	// 3. Calculate SHA256 checksum
	hasher := sha256.New()
	hasher.Write(verifiableData)
	checksum := hex.EncodeToString(hasher.Sum(nil))

	// 4. Sign the verifiable data
	sig, err := SignData(privateKey, verifiableData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign voucher data: %w", err)
	}

	// 5. Update the voucher's Signature field
	v.Signature = voucher.Signature{
		Algorithm:    "ed25519",
		KeyID:        generateKeyID(privateKey.Public().(ed25519.PublicKey)),
		SignatureB64: EncodeBase64(sig),
		ChecksumSHA256: checksum,
	}

	// 6. Marshal the final signed voucher to YAML
	finalData, err := yaml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signed voucher: %w", err)
	}

	return finalData, nil
}

// generateKeyID creates a unique ID for a public key by hashing it.
func generateKeyID(publicKey ed25519.PublicKey) string {
	hash := sha256.Sum256(publicKey)
	return hex.EncodeToString(hash[:])
}
