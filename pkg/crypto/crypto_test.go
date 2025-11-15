package crypto_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
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

func TestSignVoucher_EndToEnd(t *testing.T) {
	// 1. Setup keys
	tempDir := t.TempDir()
	privateKeyPath := filepath.Join(tempDir, "test.key")
	publicKeyPath := filepath.Join(tempDir, "test.pub")

	err := crypto.GenerateKeyPair(privateKeyPath, publicKeyPath)
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	privateKey, err := crypto.LoadPrivateKey(privateKeyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey failed: %v", err)
	}

	publicKey, err := crypto.LoadPublicKey(publicKeyPath)
	if err != nil {
		t.Fatalf("LoadPublicKey failed: %v", err)
	}

	// 2. Create a voucher with an unsorted map
	v := voucher.Voucher{
		MimicVersion: "1.0",
		Command: voucher.Command{
			Argv: []string{"echo", "hello"},
			Env: map[string]string{
				"Z_VAR": "last",
				"A_VAR": "first",
				"M_VAR": "middle",
			},
		},
		ExitCode: 0,
	}

	// 3. Sign the voucher
	signedData, err := crypto.SignVoucher(v, privateKey)
	if err != nil {
		t.Fatalf("SignVoucher failed: %v", err)
	}

	// 4. Unmarshal the signed data to simulate verification
	var signedVoucher voucher.Voucher
	if err := yaml.Unmarshal(signedData, &signedVoucher); err != nil {
		t.Fatalf("Failed to unmarshal signed voucher: %v", err)
	}

	// 5. Verify the voucher (simulating the logic from verify/replay commands)
	// a. Get the canonical representation
	canonical := crypto.GetCanonicalVoucher(signedVoucher)
	canonical.Signature = voucher.Signature{} // Clear signature for verification

	// b. Marshal to get the verifiable data
	verifiableData, err := yaml.Marshal(canonical)
	if err != nil {
		t.Fatalf("Failed to marshal canonical voucher for verification: %v", err)
	}

	// c. Verify checksum
	hasher := sha256.New()
	hasher.Write(verifiableData)
	calculatedChecksum := hex.EncodeToString(hasher.Sum(nil))
	if calculatedChecksum != signedVoucher.Signature.ChecksumSHA256 {
		t.Fatalf("Checksum verification failed: expected %s, got %s", signedVoucher.Signature.ChecksumSHA256, calculatedChecksum)
	}

	// d. Verify signature
	signatureBytes, err := crypto.DecodeBase64(signedVoucher.Signature.SignatureB64)
	if err != nil {
		t.Fatalf("Failed to decode signature: %v", err)
	}

	if !crypto.VerifySignature(publicKey, verifiableData, signatureBytes) {
		t.Fatal("Signature verification failed")
	}
}
