package recorder_test

import (
	"encoding/base64"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
)

func TestRecord(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "test.vcr")

	// Test case 1: Simple command with stdout
	cmdArgs1 := []string{"echo", "hello world"}
	voucher1, err := recorder.Record("test-version", "echo hello world", cmdArgs1, outputFile, nil, 0, false, []string{})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	if voucher1 == nil {
		t.Fatal("Voucher is nil")
	}

	// Check metadata
	expectedHostname, _ := os.Hostname()
	currentUser, _ := user.Current()
	expectedUsername := currentUser.Username

	if voucher1.Metadata.Hostname != expectedHostname {
		t.Errorf("Expected hostname '%s', got '%s'", expectedHostname, voucher1.Metadata.Hostname)
	}
	if voucher1.Metadata.User != expectedUsername {
		t.Errorf("Expected user '%s', got '%s'", expectedUsername, voucher1.Metadata.User)
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

	voucher2, err := recorder.Record("test-version", "bash -c 'echo 'error message' >&2; exit 1'", cmdArgs2, outputFile, nil, 0, false, []string{}) // Overwrite for simplicity in test, real use would be new file
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
	_, err = recorder.Record("test-version", "nonexistent-command", cmdArgs3, outputFile, nil, 0, false, []string{})
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
	voucher4, err := recorder.Record("test-version", "bash -c 'echo $MIMIC_TEST_VAR'", cmdArgs4, outputFile, nil, 0, false, []string{})
	if err != nil {
		t.Fatalf("Record with env failed: %v", err)
	}
	if voucher4.Command.Env["MIMIC_TEST_VAR"] != "hello env" {
		t.Errorf("Expected env var MIMIC_TEST_VAR to be 'hello env', got '%s'", voucher4.Command.Env["MIMIC_TEST_VAR"])
	}

	// Test case 5: With timing preservation
	cmdArgs5 := []string{"bash", "-c", "echo -n a; sleep 0.1; echo -n b"}
	voucher5, err := recorder.Record("test-version", "bash -c 'echo -n a; sleep 0.1; echo -n b'", cmdArgs5, outputFile, nil, 0, true, []string{})
	if err != nil {
		t.Fatalf("Record with timing failed: %v", err)
	}
	if len(voucher5.Stdout) < 2 {
		t.Fatalf("Expected at least 2 stdout chunks for timing test, got %d", len(voucher5.Stdout))
	}
	// Check for at least 100ms delay (100,000,000 ns)
	if voucher5.Stdout[1].DelayNs < 100*1000*1000 {
		t.Errorf("Expected at least 100ms delay between chunks, got %dms", voucher5.Stdout[1].DelayNs/1000/1000)
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

func TestRedactEnvVars(t *testing.T) {
	input := map[string]string{
		"API_KEY":           "supersecretkey",
		"GITHUB_TOKEN":      "ghp_token",
		"DB_PASSWORD":       "mypassword",
		"NON_SENSITIVE_VAR": "somevalue",
		"SECRET_STUFF":      "hidden",
		"USER":              "gregory",
		"ANOTHER_SECRET":    "shhh",
	}

	patterns := []string{
		"supersecretkey",
		"ghp_token",
		"mypassword",
		"hidden",
	}

	expectedRedaction := "[REDACTED]"

	redacted, err := recorder.RedactEnvVars(input, patterns)
	if err != nil {
		t.Fatalf("RedactEnvVars failed: %v", err)
	}

	// Check sensitive variables are redacted
	if redacted["API_KEY"] != expectedRedaction {
		t.Errorf("API_KEY: Expected '%s', got '%s'", expectedRedaction, redacted["API_KEY"])
	}
	if redacted["GITHUB_TOKEN"] != expectedRedaction {
		t.Errorf("GITHUB_TOKEN: Expected '%s', got '%s'", expectedRedaction, redacted["GITHUB_TOKEN"])
	}
	if redacted["DB_PASSWORD"] != expectedRedaction {
		t.Errorf("DB_PASSWORD: Expected '%s', got '%s'", expectedRedaction, redacted["DB_PASSWORD"])
	}
	if redacted["SECRET_STUFF"] != expectedRedaction {
		t.Errorf("SECRET_STUFF: Expected '%s', got '%s'", expectedRedaction, redacted["SECRET_STUFF"])
	}
	if redacted["ANOTHER_SECRET"] != "shhh" {
		t.Errorf("ANOTHER_SECRET: Expected 'shhh', got '%s'", redacted["ANOTHER_SECRET"])
	}

	// Check non-sensitive variables are preserved
	if redacted["NON_SENSITIVE_VAR"] != "somevalue" {
		t.Errorf("NON_SENSITIVE_VAR: Expected 'somevalue', got '%s'", redacted["NON_SENSITIVE_VAR"])
	}
	if redacted["USER"] != "gregory" {
		t.Errorf("USER: Expected 'gregory', got '%s'", redacted["USER"])
	}

	// Test case for invalid regex pattern
	invalidPatterns := []string{"["}
	_, err = recorder.RedactEnvVars(input, invalidPatterns)
	if err == nil {
		t.Errorf("Expected an error for invalid regex pattern, got none")
	}
}

func TestRecordAndReplayIntegration(t *testing.T) {
	// 1. Record a simple command
	tmpFile := filepath.Join(t.TempDir(), "test.vcr")
	v, err := recorder.Record(
		"1.0",
		"echo hello",
		[]string{"bash", "-c", "echo hello"},
		tmpFile,
		nil,
		0,
		false,
		nil,
	)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}
	if v == nil {
		t.Fatal("Voucher is nil")
	}
	if v.ExitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", v.ExitCode)
	}

	// 2. Replay
	// We need to import replayer for this
	// For now, we will just check the voucher content
	if len(v.Stdout) != 1 {
		t.Fatalf("Expected 1 stdout chunk, got %d", len(v.Stdout))
	}
	if strings.TrimSpace(readBase64(t, v.Stdout[0].DataB64)) != "hello" {
		t.Errorf("Expected stdout 'hello', got '%s'", readBase64(t, v.Stdout[0].DataB64))
	}
}

func TestEmptyChunks(t *testing.T) {
	// Voucher with no stdout/stderr
	v := voucher.Voucher{
		Stdout:   []voucher.OutputChunk{},
		Stderr:   []voucher.OutputChunk{},
		ExitCode: 0,
	}

	// Should not crash
	canonical := crypto.GetCanonicalVoucher(v)
	data, err := yaml.Marshal(canonical)
	if err != nil {
		t.Fatalf("Failed to marshal voucher with empty chunks: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Marshalled data is empty")
	}
}

func TestLargeEnvVars(t *testing.T) {
	// Create voucher with large env (>1MB)
	largeValue := strings.Repeat("x", 1000000)
	v := voucher.Voucher{
		Command: voucher.Command{
			Env: map[string]string{
				"LARGE_VAR": largeValue,
			},
		},
	}

	// Should canonicalize without issues
	canonical := crypto.GetCanonicalVoucher(v)
	data, err := yaml.Marshal(canonical)
	if err != nil {
		t.Fatalf("Failed to marshal voucher with large env var: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Marshalled data is empty")
	}
}

func TestMalformedBase64(t *testing.T) {
	v := voucher.Voucher{
		Stdout: []voucher.OutputChunk{
			{DataB64: "not-valid-base64!!!"},
		},
	}

	// Decode should fail gracefully
	_, err := voucher.DecodeChunkData(v.Stdout[0].DataB64)
	if err == nil {
		t.Fatal("Expected an error for malformed base64, got none")
	}
}
