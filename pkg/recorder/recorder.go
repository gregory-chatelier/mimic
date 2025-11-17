package recorder

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
)

func setupEnvironment(envVarsToCapture []string) []string {
	if len(envVarsToCapture) > 0 {
		var env []string
		for _, key := range envVarsToCapture {
			if value, ok := os.LookupEnv(key); ok {
				env = append(env, key+"="+value)
			}
		}
		return env
	}
	return os.Environ()
}

func setupWriters(preserveTiming bool, stdoutChunks *[]voucher.OutputChunk, stderrChunks *[]voucher.OutputChunk, stdoutBuf *bytes.Buffer, stderrBuf *bytes.Buffer) (io.Writer, io.Writer, hash.Hash) {
	var recordStdoutWriter, recordStderrWriter io.Writer
	if preserveTiming {
		recordStdoutWriter = NewTimedChunkWriter(stdoutChunks, stdoutBuf)
		recordStderrWriter = NewTimedChunkWriter(stderrChunks, stderrBuf)
	} else {
		recordStdoutWriter = stdoutBuf
		recordStderrWriter = stderrBuf
	}
	outputHasher := sha256.New()
	return recordStdoutWriter, recordStderrWriter, outputHasher
}

func executeCommand(cmd *exec.Cmd) (int, time.Duration, error) {
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	var exitCode int
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return -1, duration, fmt.Errorf("command execution failed with non-exit error: %w", err)
		}
	}
	return exitCode, duration, nil
}

func buildVoucher(mimicVersion, rawCommand string, command []string, cwd string, envMap map[string]string, stdoutChunks, stderrChunks []voucher.OutputChunk, exitCode int, duration time.Duration, ttl time.Duration, preserveTiming bool, finalOutputHash, hostname, username string) *voucher.Voucher {
	return &voucher.Voucher{
		MimicVersion: mimicVersion,
		RecordedAt:   time.Now(),
		DurationNs:   duration.Nanoseconds(),
		Command: voucher.Command{
			Raw:  rawCommand,
			Argv: command,
			Cwd:  cwd,
			Env:  envMap,
		},
		Stdout:         stdoutChunks,
		Stderr:         stderrChunks,
		ExitCode:       exitCode,
		TTL:            ttl,
		PreserveTiming: preserveTiming,
		Metadata: voucher.Metadata{
			SHA256Output: finalOutputHash,
			Hostname:     hostname,
			User:         username,
		},
	}
}

// Record executes a command and records its output and metadata to a voucher file.
// The mimicVersion parameter is used to store the version of the mimic tool that created the voucher.
func Record(mimicVersion string, rawCommand string, command []string, outputFile string, envVarsToCapture []string, ttl time.Duration, preserveTiming bool, redactPatterns []string) (*voucher.Voucher, error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = setupEnvironment(envVarsToCapture)

	var stdoutBuf, stderrBuf bytes.Buffer
	var stdoutChunks, stderrChunks []voucher.OutputChunk
	recordStdoutWriter, recordStderrWriter, outputHasher := setupWriters(preserveTiming, &stdoutChunks, &stderrChunks, &stdoutBuf, &stderrBuf)

	hashingStdoutWriter := NewHashingWriter(recordStdoutWriter, outputHasher)
	hashingStderrWriter := NewHashingWriter(recordStderrWriter, outputHasher)
	cmd.Stdout = io.MultiWriter(os.Stdout, hashingStdoutWriter)
	cmd.Stderr = io.MultiWriter(os.Stderr, hashingStderrWriter)

	exitCode, duration, err := executeCommand(cmd)
	if err != nil {
		return nil, err
	}

	if !preserveTiming {
		if stdoutBuf.Len() > 0 {
			stdoutChunks = append(stdoutChunks, voucher.OutputChunk{
				DelayNs: 0,
				DataB64: base64.StdEncoding.EncodeToString(stdoutBuf.Bytes()),
			})
		}
		if stderrBuf.Len() > 0 {
			stderrChunks = append(stderrChunks, voucher.OutputChunk{
				DelayNs: 0,
				DataB64: base64.StdEncoding.EncodeToString(stderrBuf.Bytes()),
			})
		}
	}

	envMap := make(map[string]string)
	for _, e := range cmd.Env {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			envMap[pair[0]] = pair[1]
		}
	}

	if len(redactPatterns) > 0 {
		var err error
		envMap, err = RedactEnvVars(envMap, redactPatterns)
		if err != nil {
			return nil, fmt.Errorf("redacting environment variables: %w", err)
		}
	}

	finalOutputHash := hex.EncodeToString(outputHasher.Sum(nil))

	// Get hostname and user information
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	currentUser, err := user.Current()
	username := "unknown"
	if err == nil {
		username = currentUser.Username
	}

	v := buildVoucher(mimicVersion, rawCommand, command, cmd.Dir, envMap, stdoutChunks, stderrChunks, exitCode, duration, ttl, preserveTiming, finalOutputHash, hostname, username)

	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal voucher to YAML: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write voucher to file %s: %w", outputFile, err)
	}
	if err := os.Chmod(outputFile, 0644); err != nil {
		return nil, fmt.Errorf("failed to set permissions for voucher file %s: %w", outputFile, err)
	}

	return v, nil
}

// TimedChunkWriter is an io.Writer that captures data into timed chunks.
type TimedChunkWriter struct {
	chunks    *[]voucher.OutputChunk
	buffer    *bytes.Buffer
	mu        sync.Mutex
	lastWrite time.Time
}

// NewTimedChunkWriter creates a new TimedChunkWriter.
func NewTimedChunkWriter(chunks *[]voucher.OutputChunk, buffer *bytes.Buffer) *TimedChunkWriter {
	return &TimedChunkWriter{
		chunks:    chunks,
		buffer:    buffer,
		lastWrite: time.Now(),
	}
}

// Write captures the written data as a timed chunk.
func (w *TimedChunkWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	delay := now.Sub(w.lastWrite)
	w.lastWrite = now

	// Create a new chunk
	chunk := voucher.OutputChunk{
		DelayNs: delay.Nanoseconds(),
		DataB64: base64.StdEncoding.EncodeToString(p),
	}
	*w.chunks = append(*w.chunks, chunk)

	// Also write to the buffer for non-timed access if needed
	w.buffer.Write(p)

	return len(p), nil
}

// RedactEnvVars masks sensitive environment variable values based on provided patterns.
func RedactEnvVars(envVars map[string]string, patterns []string) (map[string]string, error) {
	if len(patterns) == 0 {
		return envVars, nil
	}

	redacted := make(map[string]string, len(envVars))
	redactionString := "[REDACTED]"

	// Compile all patterns once
	var regexes []*regexp.Regexp
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to compile redaction pattern '%s': %w", p, err)
		}
		regexes = append(regexes, r)
	}

	// Iterate over a copy of the map to avoid issues with map iteration
	for k, v := range envVars {
		redactedValue := v
		for _, r := range regexes {
			redactedValue = r.ReplaceAllString(redactedValue, redactionString)
		}
		redacted[k] = redactedValue
	}

	return redacted, nil
}

// HashingWriter is an io.Writer that writes to an underlying writer and also updates a SHA256 hasher.
type HashingWriter struct {
	writer io.Writer
	hasher hash.Hash
}

// NewHashingWriter creates a new HashingWriter.
func NewHashingWriter(writer io.Writer, hasher hash.Hash) *HashingWriter {
	return &HashingWriter{
		writer: writer,
		hasher: hasher,
	}
}

// Write writes p to the underlying writer and the hasher.
func (hw *HashingWriter) Write(p []byte) (n int, err error) {
	// Write to the underlying writer
	n, err = hw.writer.Write(p)
	if err != nil {
		return n, err
	}

	// Write to the hasher (always writes all bytes, so no need to check n)
	hw.hasher.Write(p)

	return n, err
}
