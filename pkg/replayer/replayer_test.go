package replayer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

)

// Helper function to build the test helper binary
func buildTestHelper(t *testing.T, tempDir string) string {
	// We need to build the main mimic binary to test the fallback logic, as it uses cobra flags.
	testBin := filepath.Join(tempDir, "mimic.test")
	if runtime.GOOS == "windows" {
		testBin += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", testBin, ".")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build main mimic binary: %v\nOutput: %s", err, string(output))
	}
	return testBin
}

// Helper function to run the test helper binary
func runReplayTest(t *testing.T, testBin, voucherFile string, expectedExitCode int, expectedStdout, expectedStderr string, args ...string) {
	allArgs := append([]string{"replay", voucherFile}, args...)
	replayCmd := exec.Command(testBin, allArgs...)

	var stdout, stderr strings.Builder
	replayCmd.Stdout = &stdout
	replayCmd.Stderr = &stderr

	err := replayCmd.Run()

	// Check the exit code
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() != expectedExitCode {
			t.Errorf("Expected exit code %d, got %d", expectedExitCode, exitError.ExitCode())
		}
	} else if err != nil && expectedExitCode != 0 {
		t.Fatalf("Replay command failed unexpectedly: %v", err)
	} else if err == nil && expectedExitCode != 0 {
		t.Errorf("Expected non-zero exit code %d, got 0", expectedExitCode)
	}

	// Check stdout
	if stdout.String() != expectedStdout {
		t.Errorf("Expected stdout '%s', got '%s'", expectedStdout, stdout.String())
	}

	// Check stderr
	if stderr.String() != expectedStderr {
		t.Errorf("Expected stderr '%s', got '%s'", expectedStderr, stderr.String())
	}
}

func TestReplay(t *testing.T) {
	tempDir := t.TempDir()
	testBin := buildTestHelper(t, tempDir)

	// --- Test Case 1: Standard Replay ---
	voucherFile1 := filepath.Join(tempDir, "test1.vcr")
	voucherContent1 := `
	mimic_version: "1.0"
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
		runReplayTest(t, testBin, voucherFile1, 42, "hello replayer\n", "error\n")
	})

	// --- Test Case 2: Missing Voucher (Should Fail without Fallback) ---
	t.Run("Missing Voucher (Should Fail)", func(t *testing.T) {
		missingFile := filepath.Join(tempDir, "missing.vcr")
		// The main binary will fail to find the file and exit with code 1
		runReplayTest(t, testBin, missingFile, 1, "", "Error: Voucher file not found at "+missingFile+"\n")
	})
}

func TestReplayFallback(t *testing.T) {
	tempDir := t.TempDir()
	testBin := buildTestHelper(t, tempDir)

	// --- Test Case 3: Missing Voucher with Fallback (Cache Miss) ---
	t.Run("Cache Miss with Fallback", func(t *testing.T) {
		voucherFile := filepath.Join(tempDir, "fallback_miss.vcr")
		fallbackCmd := "echo 'FALLBACK EXECUTED' && exit 10"

		// 1. Run replay with fallback (Voucher is missing)
		runReplayTest(t, testBin, voucherFile, 10,
			"FALLBACK EXECUTED\n",
			"Cache is stale or missing. Executing fallback command: echo 'FALLBACK EXECUTED' && exit 10\nVoucher cache refreshed from fallback command and saved to "+voucherFile+".\n",
			"--fallback", fallbackCmd)

		// 2. Run replay again (Voucher is now present - Cache Hit)
		runReplayTest(t, testBin, voucherFile, 10,
			"FALLBACK EXECUTED\n",
			"Voucher is valid. Replaying from cache (Fallback ignored).\n")
	})

	// --- Test Case 4: Expired Voucher with Fallback (Cache Stale) ---
	t.Run("Cache Stale with Fallback", func(t *testing.T) {
		voucherFile := filepath.Join(tempDir, "fallback_stale.vcr")
		fallbackCmd := "echo 'FALLBACK REFRESHED' && exit 20"

		// 1. Create an expired voucher
		expiredContent := `
		mimic_version: "1.0"
		recorded_at: "2000-01-01T00:00:00Z" # Long ago
		duration_ms: 100
		command:
		  argv: ["echo", "expired"]
		  cwd: "/tmp"
stdout:
  - data_b64: "ZXhwaXJlZAp" # "expired\n"
stderr: []
exit_code: 0
ttl: 1s
`
		if err := os.WriteFile(voucherFile, []byte(expiredContent), 0644); err != nil {
			t.Fatalf("Failed to write expired voucher file: %v", err)
		}

		// 2. Run replay with --validate and --fallback (Voucher is stale)
		runReplayTest(t, testBin, voucherFile, 20,
			"FALLBACK REFRESHED\n",
			"Warning: Voucher has expired (Recorded at 2000-01-01T00:00:00Z, TTL 1s). Treating as cache stale.\nCache is stale or missing. Executing fallback command: echo 'FALLBACK REFRESHED' && exit 20\nVoucher cache refreshed from fallback command and saved to "+voucherFile+".\n",
			"--validate", "--fallback", fallbackCmd)

		// 3. Run replay again (Voucher is now fresh - Cache Hit)
		runReplayTest(t, testBin, voucherFile, 20,
			"FALLBACK REFRESHED\n",
			"Voucher is valid. Replaying from cache (Fallback ignored).\n")
	})
}