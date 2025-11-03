package crypto_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
)

func TestKeyGeneration(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath := filepath.Join(tempDir, "test.key")
	publicKeyPath := filepath.Join(tempDir, "test.pub")

	err := crypto.GenerateKeyPair(privateKeyPath, publicKeyPath)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		t.Errorf("Private key file was not created.")
	}
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		t.Errorf("Public key file was not created.")
	}
}

func TestSignAndVerify(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath := filepath.Join(tempDir, "test.key")
	publicKeyPath := filepath.Join(tempDir, "test.pub")

	err := crypto.GenerateKeyPair(privateKeyPath, publicKeyPath)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	pk, err := crypto.LoadPrivateKey(privateKeyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey failed: %v", err)
	}

	pub, err := crypto.LoadPublicKey(publicKeyPath)
	if err != nil {
		t.Fatalf("LoadPublicKey failed: %v", err)
	}

	data := []byte("hello world")

	sig, err := crypto.SignData(pk, data)
	if err != nil {
		t.Fatalf("SignData failed: %v", err)
	}

	if !crypto.VerifySignature(pub, data, sig) {
		t.Errorf("VerifySignature failed for valid signature.")
	}

	if crypto.VerifySignature(pub, []byte("wrong data"), sig) {
		t.Errorf("VerifySignature succeeded for invalid data.")
	}
}

func TestLoadInvalidKeys(t *testing.T) {
	// Test loading non-existent keys
	_, err := crypto.LoadPrivateKey("non-existent-file")
	if err == nil {
		t.Errorf("Expected an error for non-existent private key, got none")
	}

	_, err = crypto.LoadPublicKey("non-existent-file")
	if err == nil {
		t.Errorf("Expected an error for non-existent public key, got none")
	}

	// Test loading invalid key files
	tempDir := t.TempDir()
	invalidKeyFile := filepath.Join(tempDir, "invalid.key")
	if err := os.WriteFile(invalidKeyFile, []byte("invalid key data"), 0644); err != nil {
		t.Fatalf("Failed to write invalid key file: %v", err)
	}

	_, err = crypto.LoadPrivateKey(invalidKeyFile)
	if err == nil {
		t.Errorf("Expected an error for invalid private key file, got none")
	}

	_, err = crypto.LoadPublicKey(invalidKeyFile)
	if err == nil {
		t.Errorf("Expected an error for invalid public key file, got none")
	}
}
