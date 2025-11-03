package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
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
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %s: %w", path, err)
	}

	pemBlock, _ := pem.Decode(keyBytes)
	if pemBlock == nil || pemBlock.Type != "ED25519 PRIVATE KEY" {
		return nil, fmt.Errorf("invalid PEM block in private key file %s", path)
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

	pemBlock, _ := pem.Decode(keyBytes)
	if pemBlock == nil || pemBlock.Type != "ED25519 PUBLIC KEY" {
		return nil, fmt.Errorf("invalid PEM block in public key file %s", path)
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
