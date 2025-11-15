package recorder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// Helper function to read a voucher file and unmarshal it.
func readVoucher(t *testing.T, path string) *voucher.Voucher {
	data, err := os.ReadFile(path)
	assert.NoError(t, err)

	var v voucher.Voucher
	err = yaml.Unmarshal(data, &v)
	assert.NoError(t, err)

	return &v
}

func TestRecord_EnvVarFiltering(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "test_env_filter.vcr")

	// Set up environment variables for the test
	t.Setenv("MIMIC_TEST_VAR_1", "value1")
	t.Setenv("MIMIC_TEST_VAR_2", "value2")
	t.Setenv("MIMIC_TEST_SECRET", "sensitive")

	cmdArgs := []string{"echo", "hello"}
	// Specify to only capture MIMIC_TEST_VAR_1 and MIMIC_TEST_VAR_2
	envVarsToCapture := []string{"MIMIC_TEST_VAR_1", "MIMIC_TEST_VAR_2"}

	_, err := recorder.Record(cmdArgs, outputFile, envVarsToCapture, 0, false, nil)
	assert.NoError(t, err)

	v := readVoucher(t, outputFile)

	// Assert that only the specified environment variables were recorded
	assert.Len(t, v.Command.Env, 2, "Should only have 2 environment variables")
	assert.Equal(t, "value1", v.Command.Env["MIMIC_TEST_VAR_1"])
	assert.Equal(t, "value2", v.Command.Env["MIMIC_TEST_VAR_2"])
	assert.NotContains(t, v.Command.Env, "MIMIC_TEST_SECRET", "Should not contain the secret variable")
}

func TestRecord_EnvVarRedaction(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "test_redaction.vcr")

	// Set up environment variables
	t.Setenv("API_KEY", "abc-123-def-456")
	t.Setenv("PASSWORD", "my-secret-password")
	t.Setenv("USER", "test-user")

	cmdArgs := []string{"bash", "-c", "echo $API_KEY"}
	// Capture all env vars, but redact the sensitive ones
	redactPatterns := []string{
		"abc-123-def-456",
		"my-secret-password",
	}

	_, err := recorder.Record(cmdArgs, outputFile, nil, 0, false, redactPatterns)
	assert.NoError(t, err)

	v := readVoucher(t, outputFile)

	// 1. Check that environment variables are redacted in the voucher
	assert.Equal(t, "[REDACTED]", v.Command.Env["API_KEY"])
	assert.Equal(t, "[REDACTED]", v.Command.Env["PASSWORD"])
	assert.Equal(t, "test-user", v.Command.Env["USER"], "Non-redacted variable should be unchanged")

	// 2. Check that the command's stdout was NOT redacted
	assert.NotEmpty(t, v.Stdout, "Stdout should not be empty")
	stdout, err := v.GetDecodedStdout()
	assert.NoError(t, err)
	// The actual output of `echo $API_KEY` should be the original, unredacted value
	assert.Equal(t, "abc-123-def-456\n", string(stdout), "Stdout should contain the original, unredacted value")
}

func TestRecord_NoEnvVars(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "test_no_env.vcr")

	cmdArgs := []string{"echo", "no env test"}

	// Pass an empty slice for envVarsToCapture, which should default to all
	_, err := recorder.Record(cmdArgs, outputFile, []string{}, 0, false, nil)
	assert.NoError(t, err)

	v := readVoucher(t, outputFile)

	// Should capture all environment variables of the current process
	assert.True(t, len(v.Command.Env) > 0, "Should capture all env vars if slice is empty")
	assert.Contains(t, v.Command.Env, "PATH", "Should contain common env vars like PATH")
}

func TestRecord_RedactionDoesNotAffectStdout(t *testing.T) {
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "test_stdout_unredacted.vcr")

	// This variable will be printed to stdout and also stored (and redacted) in the env
	t.Setenv("SENSITIVE_DATA", "this-is-secret")

	cmdArgs := []string{"bash", "-c", "echo $SENSITIVE_DATA"}
	redactPatterns := []string{"this-is-secret"}

	_, err := recorder.Record(cmdArgs, outputFile, nil, 0, false, redactPatterns)
	assert.NoError(t, err)

	v := readVoucher(t, outputFile)

	// 1. Verify the env var is redacted
	assert.Equal(t, "[REDACTED]", v.Command.Env["SENSITIVE_DATA"])

	// 2. Verify stdout is NOT redacted
	stdout, err := v.GetDecodedStdout()
	assert.NoError(t, err)
	assert.Equal(t, "this-is-secret\n", string(stdout))
}