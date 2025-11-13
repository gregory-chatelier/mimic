package replayer_test

import (
	"bytes"
	"fmt" // Import fmt for Sprintf
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregory-chatelier/mimic/cmd" // Import the cmd package
)

// Helper function to capture stdout/stderr
func captureOutput(f func(), stdout, stderr *bytes.Buffer) {
	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	// Save original stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	// Redirect stdout and stderr to our pipes
	os.Stdout = wOut
	os.Stderr = wErr

	// Run the function in a goroutine to allow output capture
	outChan := make(chan struct{})
	errChan := make(chan struct{})

	go func() {
		io.Copy(stdout, rOut)
		rOut.Close()
		close(outChan)
	}()

	go func() {
		io.Copy(stderr, rErr)
		rErr.Close()
		close(errChan)
	}()

	f()

	// Close the write ends of the pipes to signal EOF to the readers
	wOut.Close()
	wErr.Close()

	// Wait for the goroutines to finish reading
	<-outChan
	<-errChan

	// Restore original stdout and stderr
	os.Stdout = oldStdout
	os.Stderr = oldStderr
}

func TestReplay(t *testing.T) {
	tempDir := t.TempDir()

	// --- Test Case 1: Standard Replay ---
	voucherFile1 := filepath.Join(tempDir, "test1.vcr")
	voucherContent1 := `mimic_version: "1.0"
recorded_at: "2025-11-03T10:00:00Z"
duration_ms: 100
command:
  argv: ["echo", "hello replayer"]
  cwd: "/tmp"
stdout:
  - data_b64: "aGVsbG8gcmVwbGF5ZXIK" # "hello replayer\n"
stderr:
  - data_b64: "ZXJyb3IK" # "error\n"
exit_code: 42
`
	if err := os.WriteFile(voucherFile1, []byte(voucherContent1), 0644); err != nil {
		t.Fatalf("Failed to write dummy voucher file: %v", err)
	}

	t.Run("Standard Replay", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exitCode := 0
		var err error

		captureOutput(func() {
			exitCode, err = cmd.RunReplayCommand(voucherFile1, nil, false, "", "", false, 1.0, false)
		}, &stdout, &stderr)

		if err != nil {
			t.Errorf("runReplayCommand returned an unexpected error: %v", err)
		}
		if exitCode != 42 {
			t.Errorf("Expected exit code 42, got %d", exitCode)
		}
		if stdout.String() != "hello replayer\n" {
			t.Errorf("Expected stdout 'hello replayer\\n', got '%s'", stdout.String())
		}
		if stderr.String() != "Voucher is valid. Replaying from cache.\nerror\n" {
			t.Errorf("Expected stderr 'Voucher is valid. Replaying from cache.\\nerror\\n', got '%s'", stderr.String())
		}
	})

	// --- Test Case 2: Missing Voucher (Should Fail without Fallback) ---
	t.Run("Missing Voucher (Should Fail)", func(t *testing.T) {
		missingFile := filepath.Join(tempDir, "missing.vcr")
		var stdout, stderr bytes.Buffer
		exitCode := 0
		var err error

		captureOutput(func() {
			exitCode, err = cmd.RunReplayCommand(missingFile, nil, false, "", "", false, 1.0, false)
		}, &stdout, &stderr)

		if err == nil {
			t.Errorf("runReplayCommand did not return an error for missing voucher")
		}
		if exitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", exitCode)
		}
		expectedErr := fmt.Sprintf("Voucher file not found at '%s'", missingFile)
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedErr, err.Error())
		}
		if stdout.String() != "" {
			t.Errorf("Expected empty stdout, got '%s'", stdout.String())
		}
		if stderr.String() != "" { // Error is returned, not printed to stderr by runReplayCommand
			t.Errorf("Expected empty stderr, got '%s'", stderr.String())
		}
	})
}

func TestReplayFallback(t *testing.T) {
	tempDir := t.TempDir()

	// --- Test Case 3: Missing Voucher with Fallback (Cache Miss) ---
	t.Run("Cache Miss with Fallback", func(t *testing.T) {
		voucherFile := filepath.Join(tempDir, "fallback_miss.vcr")
		fallbackCmdToExecute := []string{"bash", "-c", "echo 'FALLBACK EXECUTED'; exit 10"}
		
		var stdout, stderr bytes.Buffer
		exitCode := 0
		var err error

		captureOutput(func() {
			exitCode, err = cmd.RunReplayCommand(voucherFile, fallbackCmdToExecute, false, "", "", false, 1.0, true)
		}, &stdout, &stderr)

		if err != nil {
			t.Errorf("runReplayCommand returned an unexpected error: %v", err)
		}
		if exitCode != 10 { // Replay calls os.Exit, so runReplayCommand returns 0
			t.Errorf("Expected exit code 10, got %d", exitCode)
		}
		expectedStdout := "FALLBACK EXECUTED\n"
		if stdout.String() != expectedStdout {
			t.Errorf("Expected stdout '%s', got '%s'", expectedStdout, stdout.String())
		}
		if !strings.Contains(stderr.String(), "Cache is stale or missing. Executing fallback command: bash -c echo 'FALLBACK EXECUTED'; exit 10\nVoucher recorded to ") || !strings.Contains(stderr.String(), "Voucher cache refreshed from fallback command and saved to "+voucherFile+"\n") {
			t.Errorf("Expected stderr to contain fallback messages, got '%s'", stderr.String())
		}
	})

	// --- Test Case 4: Expired Voucher with Fallback (Cache Stale) ---
	t.Run("Cache Stale with Fallback", func(t *testing.T) {
		voucherFile := filepath.Join(tempDir, "fallback_stale.vcr")
		fallbackCmdToExecute := []string{"bash", "-c", "echo 'FALLBACK REFRESHED' ; exit 20"}

		// 1. Create an expired voucher
		expiredContent := `mimic_version: "1.0"
recorded_at: "2000-01-01T00:00:00Z" # Long ago
duration_ms: 100
command:
  argv: ["echo", "expired"]
  cwd: "/tmp"
stdout:
  - data_b64: "ZXhwaXJlZApK" # "expired\n"
stderr: []
exit_code: 0
ttl: 1s
`
		if err := os.WriteFile(voucherFile, []byte(expiredContent), 0644); err != nil {
			t.Fatalf("Failed to write expired voucher file: %v", err)
		}

		// Create a dummy public key file for validation
		publicKeyPath := filepath.Join(tempDir, "mimic.pub")
		dummyPublicKeyContent := `-----BEGIN ED25519 PUBLIC KEY-----
4eh+KDLD80tU5WuQ0W0IAG2X5nR1LavGy/8dzyKAcR4=
-----END ED25519 PUBLIC KEY-----`
		if err := os.WriteFile(publicKeyPath, []byte(dummyPublicKeyContent), 0644); err != nil {
			t.Fatalf("Failed to write dummy public key file: %v", err)
		}

		// Create a dummy private key file for re-signing
		privateKeyPath := filepath.Join(tempDir, "mimic.key")
		dummyPrivateKeyContent := `-----BEGIN ED25519 PRIVATE KEY-----
RCq0TC/17sDDvSJbzQq6FmnnLRKjh6gNout7Sm/AlF3h6H4oMsPzS1Tla5DRbQgA
bZfmdHUtq8bL/x3PIoBxHg==
-----END ED25519 PRIVATE KEY-----`
		if err := os.WriteFile(privateKeyPath, []byte(dummyPrivateKeyContent), 0644); err != nil {
			t.Fatalf("Failed to write dummy private key file: %v", err)
		}

		var stdout, stderr bytes.Buffer
		exitCode := 0
		var err error

		captureOutput(func() {
			exitCode, err = cmd.RunReplayCommand(voucherFile, fallbackCmdToExecute, true, publicKeyPath, privateKeyPath, false, 1.0, true)
		}, &stdout, &stderr)

		if err != nil {
			t.Errorf("runReplayCommand returned an unexpected error: %v", err)
		}
		if exitCode != 20 { // Replay calls os.Exit, so runReplayCommand returns 0
			t.Errorf("Expected exit code 20, got %d", exitCode)
		}
		expectedStdout := "FALLBACK REFRESHED\n"
		expectedStderrPrefix := fmt.Sprintf("Warning: Voucher has expired (Recorded at 2000-01-01T00:00:00Z, TTL 1s). Treating as cache stale.\nCache is stale or missing. Executing fallback command: bash -c echo 'FALLBACK REFRESHED' ; exit 20\nVoucher recorded to ")
		expectedStderrSuffix := fmt.Sprintf("\nVoucher successfully re-signed.\nVoucher cache refreshed from fallback command and saved to %s\n", voucherFile)
		
		if stdout.String() != expectedStdout {
			t.Errorf("Expected stdout '%s', got '%s'", expectedStdout, stdout.String())
		}
		if !strings.HasPrefix(stderr.String(), expectedStderrPrefix) || !strings.HasSuffix(stderr.String(), expectedStderrSuffix) || !strings.Contains(stderr.String(), "mimic-fallback-") {
			t.Errorf("Expected stderr to start with '%s', end with '%s' and contain 'mimic-fallback-', got '%s'", expectedStderrPrefix, expectedStderrSuffix, stderr.String())
		}
	})
}
