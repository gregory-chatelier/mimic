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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
)

// Record executes a command and records its output and metadata to a voucher file.
// The mimicVersion parameter is used to store the version of the mimic tool that created the voucher.
func Record(mimicVersion string, rawCommand string, command []string, outputFile string, envVarsToCapture []string, ttl time.Duration, preserveTiming bool, redactPatterns []string) (*voucher.Voucher, error) {
	cmd := exec.Command(command[0], command[1:]...)

	// Environment variable handling
	if len(envVarsToCapture) > 0 {
		for _, key := range envVarsToCapture {
			if value, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
	} else {
		cmd.Env = os.Environ()
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var stdoutChunks, stderrChunks []voucher.OutputChunk

	// Initialize SHA256 hasher for combined stdout/stderr
	outputHasher := sha256.New()

	// Create custom writers that will capture chunks
	var recordStdoutWriter, recordStderrWriter io.Writer
	if preserveTiming {
		recordStdoutWriter = NewTimedChunkWriter(&stdoutChunks, &stdoutBuf)
		recordStderrWriter = NewTimedChunkWriter(&stderrChunks, &stderrBuf)
	} else {
		recordStdoutWriter = &stdoutBuf
		recordStderrWriter = &stderrBuf
	}

	// Wrap the recording writers with HashingWriter to update the SHA256 hash
	hashingStdoutWriter := NewHashingWriter(recordStdoutWriter, outputHasher)
	hashingStderrWriter := NewHashingWriter(recordStderrWriter, outputHasher)

	// Use io.MultiWriter to tee output to both the recording mechanism and os.Stdout/os.Stderr
	cmd.Stdout = io.MultiWriter(os.Stdout, hashingStdoutWriter)
	cmd.Stderr = io.MultiWriter(os.Stderr, hashingStderrWriter)

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	var exitCode int
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("command execution failed with non-exit error: %w", err)
		}
	}

	// If not preserving timing, create single chunks from the buffers
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

	// Environment map for voucher
	envMap := make(map[string]string)
	if envVarsToCapture == nil || len(envVarsToCapture) == 0 {
		// Capture all environment variables
		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			if len(pair) == 2 {
				envMap[pair[0]] = pair[1]
			}
		}
	} else {
		// Capture only specified environment variables
		for _, key := range envVarsToCapture {
			if value, ok := os.LookupEnv(key); ok {
				envMap[key] = value
			}
		}
	}

	// Redact environment variables if patterns are provided
	if len(redactPatterns) > 0 {
		var err error
		envMap, err = RedactEnvVars(envMap, redactPatterns)
		if err != nil {
			return nil, fmt.Errorf("redacting environment variables: %w", err)
		}
	}

	// Get the final SHA256 hash of the combined output
	finalOutputHash := hex.EncodeToString(outputHasher.Sum(nil))

	// Create voucher
	v := &voucher.Voucher{
		MimicVersion: mimicVersion,
		RecordedAt:   startTime,
		DurationNs:   duration.Nanoseconds(),
		Command: voucher.Command{
			Raw:  rawCommand,
			Argv: command,
			Cwd:  cmd.Dir, // Will be empty if not set, which is fine
			Env:  envMap,
		},
		Stdout:         stdoutChunks,
		Stderr:         stderrChunks,
		ExitCode:       exitCode,
		TTL:            ttl,
		PreserveTiming: preserveTiming,
		Metadata: voucher.Metadata{
			SHA256Output: finalOutputHash,
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal voucher to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write voucher to file %s: %w", outputFile, err)
	}
	// Explicitly set permissions as os.WriteFile might not always honor them on some filesystems (e.g., WSL)
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
