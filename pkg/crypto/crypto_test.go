package crypto_test

import (
	"bytes"
	"crypto/ed25519"
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

func TestCanonicalVoucherDeterminism(t *testing.T) {
	// 1. Create voucher with unordered map
	v := voucher.Voucher{
		Command: voucher.Command{
			Env: map[string]string{
				"ZEBRA": "z",
				"ALPHA": "a",
				"MIKE":  "m",
			},
		},
	}

	// 2. Canonicalize multiple times
	results := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		canonical := crypto.GetCanonicalVoucher(v)
		data, err := yaml.Marshal(canonical)
		if err != nil {
			t.Fatalf("Failed to marshal canonical voucher: %v", err)
		}
		results[i] = data
	}

	// 3. All should be identical
	for i := 1; i < 5; i++ {
		if !bytes.Equal(results[0], results[i]) {
			t.Fatalf("canonicalization non-deterministic at iteration %d", i)
		}
	}

	// 4. Verify order is sorted
	canonical := crypto.GetCanonicalVoucher(v)
	if !(canonical.Command.Env[0].Key == "ALPHA" && canonical.Command.Env[1].Key == "MIKE" && canonical.Command.Env[2].Key == "ZEBRA") {
		t.Fatalf("env vars not sorted correctly")
	}
}

func TestTamperingDetection(t *testing.T) {
	// 1. Create and sign voucher
	v := voucher.Voucher{
		MimicVersion: "1.0",
		Command: voucher.Command{
			Raw:  "echo test",
			Argv: []string{"echo", "test"},
		},
	}
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	signedData, err := crypto.SignVoucher(v, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign voucher: %v", err)
	}

	// 2. Tamper with signed YAML
	tamperedData := bytes.ReplaceAll(signedData, []byte("echo test"), []byte("echo hack"))

	// 3. Try to verify tampered data
	var tamperedVoucher voucher.Voucher
	err = yaml.Unmarshal(tamperedData, &tamperedVoucher)
	if err != nil {
		t.Fatalf("failed to unmarshal tampered data: %v", err)
	}

	// 4. Checksum should fail
	canonical := crypto.GetCanonicalVoucher(tamperedVoucher)
	canonical.Signature = voucher.Signature{}
	verifiableData, _ := yaml.Marshal(canonical)

	hasher := sha256.New()
	hasher.Write(verifiableData)
	calculatedChecksum := hex.EncodeToString(hasher.Sum(nil))

	if calculatedChecksum == tamperedVoucher.Signature.ChecksumSHA256 {
		t.Fatal("tampering not detected by checksum")
	}
}

func TestOutputHashVerification(t *testing.T) {
	// 1. Create voucher with output
	outputData := []byte("line1\nline2\nline3\n")
	hasher := sha256.New()
	hasher.Write(outputData)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	v := voucher.Voucher{
		Stdout: []voucher.OutputChunk{
			{DataB64: crypto.EncodeBase64(outputData)},
		},
		Metadata: voucher.Metadata{
			SHA256Output: expectedHash,
		},
	}

	// 2. Verify correct hash
	// We need to create a dummy key and sign the voucher to use VerifyVoucherIntegrity
	tempDir := t.TempDir()
	privateKeyPath := filepath.Join(tempDir, "test.key")
	publicKeyPath := filepath.Join(tempDir, "test.pub")
	if err := crypto.GenerateKeyPair(privateKeyPath, publicKeyPath); err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	privateKey, err := crypto.LoadPrivateKey(privateKeyPath)
	if err != nil {
		t.Fatalf("Failed to load private key: %v", err)
	}

	signedData, err := crypto.SignVoucher(v, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign voucher: %v", err)
	}
	var signedVoucher voucher.Voucher
	yaml.Unmarshal(signedData, &signedVoucher)

	err = crypto.VerifyVoucherIntegrity(&signedVoucher, publicKeyPath)
	if err != nil {
		t.Fatalf("Verification failed for correct hash: %v", err)
	}

	// 3. Verify wrong hash fails
	signedVoucher.Metadata.SHA256Output = "wronghash"
	err = crypto.VerifyVoucherIntegrity(&signedVoucher, publicKeyPath)
	if err == nil {
		t.Fatal("Verification succeeded for wrong hash")
	}
}
