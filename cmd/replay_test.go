package cmd_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gregory-chatelier/mimic/cmd"
	"github.com/gregory-chatelier/mimic/pkg/crypto"
)

// Helper function to capture stdout/stderr
func captureOutput(t *testing.T, f func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}
	os.Stdout = wOut
	os.Stderr = wErr

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(&stdoutBuf, rOut); err != nil {
			t.Errorf("Failed to copy stdout: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(&stderrBuf, rErr); err != nil {
			t.Errorf("Failed to copy stderr: %v", err)
		}
	}()

	f() // This is where the replay command is run

	wOut.Close()
	wErr.Close()
	wg.Wait() // Wait for the readers to finish

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdoutBuf.String(), stderrBuf.String()
}

func TestReplay(t *testing.T) {
	tempDir := t.TempDir()

	// --- Test Case 1: Standard Replay ---
	voucherFile1 := filepath.Join(tempDir, "test1.vcr")
	voucherContent1 := `mimic_version: "1.0"
recorded_at: "2025-11-03T10:00:00Z"
duration_ns: 100000000
command:
  argv: ["echo", "hello replayer"]
  cwd: "/tmp"
stdout:
  - delay_ns: 0
    data_b64: "aGVsbG8gcmVwbGF5ZXIK"
stderr:
  - delay_ns: 0
    data_b64: "ZXJyb3IK"
exit_code: 42
`
	if err := os.WriteFile(voucherFile1, []byte(voucherContent1), 0644); err != nil {
		t.Fatalf("Failed to write dummy voucher file: %v", err)
	}

	t.Run("Standard Replay", func(t *testing.T) {
		var exitCode int
		var err error
		stdout, stderr := captureOutput(t, func() {
			exitCode, err = cmd.RunReplayCommand(voucherFile1, nil, false, "", "", false, false, "", false)
		})

		if err != nil {
			t.Errorf("runReplayCommand returned an unexpected error: %v", err)
		}
		if exitCode != 42 {
			t.Errorf("Expected exit code 42, got %d", exitCode)
		}
		if stdout != "hello replayer\n" {
			t.Errorf("Expected stdout 'hello replayer\\n', got '%s'", stdout)
		}
		if !strings.HasSuffix(strings.TrimSpace(stderr), "error") {
			t.Errorf("Expected stderr to end with 'error', got '%s'", stderr)
		}
	})

	// --- Test Case 2: Missing Voucher (Should Fail without Fallback) ---
	t.Run("Missing Voucher (Should Fail)", func(t *testing.T) {
		missingFile := filepath.Join(tempDir, "missing.vcr")
		var exitCode int
		var err error
		_, _ = captureOutput(t, func() {
			exitCode, err = cmd.RunReplayCommand(missingFile, nil, false, "", "", false, false, "", false)
		})

		if err == nil {
			t.Errorf("runReplayCommand did not return an error for missing voucher")
		}
		if exitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", exitCode)
		}
		expectedErr := "voucher is missing, malformed, or expired, and no fallback was provided"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("Expected error to contain '%s', got '%s'", expectedErr, err.Error())
		}
	})
}

func TestReplayFallback(t *testing.T) {
	tempDir := t.TempDir()

	// --- Test Case 3: Missing Voucher with Fallback (Cache Miss) ---
	t.Run("Cache Miss with Fallback", func(t *testing.T) {
		voucherFile := filepath.Join(tempDir, "fallback_miss.vcr")
		fallbackCmdToExecute := []string{"bash", "-c", "echo 'FALLBACK EXECUTED'; exit 10"}

		var exitCode int
		var err error
		stdout, stderr := captureOutput(t, func() {
			exitCode, err = cmd.RunReplayCommand(voucherFile, fallbackCmdToExecute, false, "", "", true, false, "", false)
		})

		if err != nil {
			t.Errorf("runReplayCommand returned an unexpected error: %v", err)
		}
		if exitCode != 10 {
			t.Errorf("Expected exit code 10, got %d", exitCode)
		}
		expectedStdout := "FALLBACK EXECUTED\nFALLBACK EXECUTED\n" // Output from live execution + replay
		if stdout != expectedStdout {
			t.Errorf("Expected stdout '%s', got '%s'", expectedStdout, stdout)
		}
		if !strings.Contains(stderr, "Cache is stale or missing. Executing fallback command:") {
			t.Errorf("Expected stderr to contain 'Cache is stale or missing', got '%s'", stderr)
		}
		// Removed: if !strings.Contains(stderr, "Voucher recorded to") {
		// 	t.Errorf("Expected stderr to contain 'Voucher recorded to', got '%s'", stderr)
		// }
		if !strings.Contains(stderr, "Voucher cache refreshed from fallback command and saved to") {
			t.Errorf("Expected stderr to contain 'Voucher cache refreshed', got '%s'", stderr)
		}
	})

	// --- Test Case 4: Expired Voucher with Fallback (Cache Stale) ---
	t.Run("Cache Stale with Fallback", func(t *testing.T) {
		voucherFile := filepath.Join(tempDir, "fallback_stale.vcr")
		fallbackCmdToExecute := []string{"bash", "-c", "echo 'FALLBACK REFRESHED' ; exit 20"}

		// 1. Create an expired voucher
		expiredContent := `mimic_version: "1.0"
recorded_at: "2000-01-01T00:00:00Z"
duration_ns: 100000000
command:
  argv: ["echo", "expired"]
  cwd: "/tmp"
stdout:
  - delay_ns: 0
    data_b64: "ZXhwaXJlZApK"
stderr: []
exit_code: 0
ttl: 1s
`

		if err := os.WriteFile(voucherFile, []byte(expiredContent), 0644); err != nil {
			t.Fatalf("Failed to write expired voucher file: %v", err)
		}

		publicKeyPath := filepath.Join(tempDir, "mimic.pub")
		privateKeyPath := filepath.Join(tempDir, "mimic.key")
		if err := crypto.GenerateKeyPair(privateKeyPath, publicKeyPath); err != nil {
			t.Fatalf("Failed to generate key pair: %v", err)
		}

		var exitCode int
		var err error
		stdout, stderr := captureOutput(t, func() {
			exitCode, err = cmd.RunReplayCommand(voucherFile, fallbackCmdToExecute, true, publicKeyPath, privateKeyPath, true, false, "", false)
		})

		if err != nil {
			t.Errorf("runReplayCommand returned an unexpected error: %v", err)
		}
		if exitCode != 20 {
			t.Errorf("Expected exit code 20, got %d", exitCode)
		}
		expectedStdout := "FALLBACK REFRESHED\nFALLBACK REFRESHED\n" // Output from live execution + replay
		if stdout != expectedStdout {
			t.Errorf("Expected stdout '%s', got '%s'", expectedStdout, stdout)
		}
		if !strings.Contains(stderr, "Warning: validation failed (voucher has expired") {
			t.Errorf("Expected stderr to contain 'Warning: validation failed (voucher has expired', got '%s'", stderr)
		}
		if !strings.Contains(stderr, "Cache is stale or missing. Executing fallback command:") {
			t.Errorf("Expected stderr to contain 'Cache is stale or missing', got '%s'", stderr)
		}
	})
}
