package recorder_test

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
)

func TestRecord(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "test.vcr")

	// Test case 1: Simple command with stdout
	cmdArgs1 := []string{"echo", "hello world"}
	voucher1, err := recorder.Record(cmdArgs1, outputFile, false, 0, false, "")
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if voucher1 == nil {
		t.Fatal("Voucher is nil")
	}

	if voucher1.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", voucher1.ExitCode)
	}

	if len(voucher1.Stdout) != 1 {
		t.Fatalf("Expected 1 stdout chunk, got %d", len(voucher1.Stdout))
	}
	if strings.TrimSpace(readBase64(t, voucher1.Stdout[0].DataB64)) != "hello world" {
		t.Errorf("Expected stdout 'hello world', got '%s'", readBase64(t, voucher1.Stdout[0].DataB64))
	}

	// Verify file content
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read voucher file: %v", err)
	}
	var loadedVoucher voucher.Voucher
	if err := yaml.Unmarshal(data, &loadedVoucher); err != nil {
		t.Fatalf("Failed to unmarshal voucher from file: %v", err)
	}
	if loadedVoucher.ExitCode != 0 {
		t.Errorf("File: Expected exit code 0, got %d", loadedVoucher.ExitCode)
	}

	// Test case 2: Command with stderr and non-zero exit code
	cmdArgs2 := []string{"bash", "-c", "echo 'error message' >&2; exit 1"}
	// Check if bash is available, otherwise skip this test
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Skipping stderr test: bash not found in PATH.")
	}

	voucher2, err := recorder.Record(cmdArgs2, outputFile, false, 0, false, "") // Overwrite for simplicity in test, real use would be new file
	if err != nil {
		t.Fatalf("Record failed for stderr test: %v", err)
	}

	if voucher2.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", voucher2.ExitCode)
	}
	if len(voucher2.Stderr) != 1 {
		t.Fatalf("Expected 1 stderr chunk, got %d", len(voucher2.Stderr))
	}
	if strings.TrimSpace(readBase64(t, voucher2.Stderr[0].DataB64)) != "error message" {
		t.Errorf("Expected stderr 'error message', got '%s'", readBase64(t, voucher2.Stderr[0].DataB64))
	}

	// Test case 3: Command not found
	cmdArgs3 := []string{"nonexistent-command"}
	_, err = recorder.Record(cmdArgs3, outputFile, false, 0, false, "")
	if err == nil {
		t.Fatalf("Expected an error for nonexistent command, got none")
	}
	// Check for a specific error message if possible, or just non-nil error
	if !strings.Contains(err.Error(), "failed to run command") && !strings.Contains(err.Error(), "executable file not found") {
		t.Errorf("Unexpected error for nonexistent command: %v", err)
	}

	// Test case 4: With environment variables
	os.Setenv("MIMIC_TEST_VAR", "hello env")
	defer os.Unsetenv("MIMIC_TEST_VAR")
	cmdArgs4 := []string{"bash", "-c", "echo $MIMIC_TEST_VAR"}
	voucher4, err := recorder.Record(cmdArgs4, outputFile, true, 0, false, "")
	if err != nil {
		t.Fatalf("Record with env failed: %v", err)
	}
	if voucher4.Command.Env["MIMIC_TEST_VAR"] != "hello env" {
		t.Errorf("Expected env var MIMIC_TEST_VAR to be 'hello env', got '%s'", voucher4.Command.Env["MIMIC_TEST_VAR"])
	}

	// Test case 5: With timing preservation
	cmdArgs5 := []string{"bash", "-c", "echo -n a; sleep 0.1; echo -n b"}
	voucher5, err := recorder.Record(cmdArgs5, outputFile, false, 0, true, "")
	if err != nil {
		t.Fatalf("Record with timing failed: %v", err)
	}
	if len(voucher5.Stdout) < 2 {
		t.Fatalf("Expected at least 2 stdout chunks for timing test, got %d", len(voucher5.Stdout))
	}
	if voucher5.Stdout[1].DelayMs < 100 {
		t.Errorf("Expected at least 100ms delay between chunks, got %dms", voucher5.Stdout[1].DelayMs)
	}
}

// Helper to read content from base64 string
func readBase64(t *testing.T, b64 string) string {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}
	return string(data)
}
