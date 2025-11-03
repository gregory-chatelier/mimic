package replayer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplay(t *testing.T) {
	// Build the test helper binary
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	cmdPath := filepath.Join(testDir, "cmd", "replay_test_main.go")
	testBin := filepath.Join(t.TempDir(), "replayer.test")

	buildCmd := exec.Command("go", "build", "-o", testBin, cmdPath)
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build test helper binary: %v\nOutput: %s", err, string(output))
	}

	// Create a dummy voucher file
	voucherFile := filepath.Join(t.TempDir(), "test.vcr")
	voucherContent := `
mimic_version: "1.0"
recorded_at: "2025-11-03T10:00:00Z"
duration_ms: 100
command:
  argv: ["echo", "hello replayer"]
  cwd: "/tmp"
stdout:
  - data_b64: "aGVsbG8gcmVwbGF5ZXIK"
stderr:
  - data_b64: "ZXJyb3IK"
exit_code: 42
`
	if err := os.WriteFile(voucherFile, []byte(voucherContent), 0644); err != nil {
		t.Fatalf("Failed to write dummy voucher file: %v", err)
	}

	// Run the test helper binary
	replayCmd := exec.Command(testBin, voucherFile)

	var stdout, stderr strings.Builder
	replayCmd.Stdout = &stdout
	replayCmd.Stderr = &stderr

	err = replayCmd.Run()

	// Check the exit code
	expectedExitCode := 42
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() != expectedExitCode {
			t.Errorf("Expected exit code %d, got %d", expectedExitCode, exitError.ExitCode())
		}
	} else if err != nil {
		t.Fatalf("Replay command failed unexpectedly: %v", err)
	}

	// Check stdout
	expectedStdout := "hello replayer\n"
	if stdout.String() != expectedStdout {
		t.Errorf("Expected stdout '%s', got '%s'", expectedStdout, stdout.String())
	}

	// Check stderr
	expectedStderr := "error\n"
	if stderr.String() != expectedStderr {
		t.Errorf("Expected stderr '%s', got '%s'", expectedStderr, stderr.String())
	}
}
