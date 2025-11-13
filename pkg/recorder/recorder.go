package recorder

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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

// TimedChunkWriter is an io.Writer that records the time and content of each write operation.
type TimedChunkWriter struct {
	mu        sync.Mutex
	chunks    []voucher.OutputChunk
	lastWrite time.Time
	buffer    bytes.Buffer
}

// NewTimedChunkWriter creates a new TimedChunkWriter.
func NewTimedChunkWriter() *TimedChunkWriter {
	return &TimedChunkWriter{
		lastWrite: time.Now(),
	}
}

// Write implements the io.Writer interface.
func (w *TimedChunkWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	delay := time.Since(w.lastWrite)
	w.chunks = append(w.chunks, voucher.OutputChunk{
		DelayMs: int(delay.Milliseconds()),
		DataB64: base64.StdEncoding.EncodeToString(p),
	})
	w.lastWrite = time.Now()
	w.buffer.Write(p)

	return len(p), nil
}

// Chunks returns the recorded output chunks.
func (w *TimedChunkWriter) Chunks() []voucher.OutputChunk {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.chunks
}

// String returns the full output as a single string.
func (w *TimedChunkWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

// RedactEnvVars masks sensitive environment variable values based on provided patterns.
func RedactEnvVars(envVars map[string]string, patterns []string) (map[string]string, error) {
	redacted := make(map[string]string, len(envVars))
	redactionString := "******** REDACTED ********"

	// Compile all patterns into a list of regex objects
	var regexes []*regexp.Regexp
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid redaction pattern '%s': %w", p, err)
		}
		regexes = append(regexes, r)
	}

	for k, v := range envVars {
		isSensitive := false
		for _, r := range regexes {
			if r.MatchString(k) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			redacted[k] = redactionString
		} else {
			redacted[k] = v
		}
	}
	return redacted, nil
}

// Record executes a command, captures its behavior, and returns a Voucher.
func Record(cmdArgs []string, outputFile string, envVarsToCapture []string, ttl time.Duration, preserveTiming bool, prevVoucherPath string, redactPatterns []string) (*voucher.Voucher, error) {
	// Separate command and arguments
	name := cmdArgs[0]
	args := cmdArgs[1:]

	cmd := exec.Command(name, args...)

	var stdoutWriter, stderrWriter io.Writer

	if preserveTiming {
		timedStdout := NewTimedChunkWriter()
		timedStderr := NewTimedChunkWriter()
		stdoutWriter = timedStdout
		stderrWriter = timedStderr
		cmd.Stdout = stdoutWriter
		cmd.Stderr = stderrWriter
	} else {
		var stdoutBuf, stderrBuf bytes.Buffer
		stdoutWriter = &stdoutBuf
		stderrWriter = &stderrBuf
		cmd.Stdout = stdoutWriter
		cmd.Stderr = stderrWriter
	}

	start := time.Now()

	err := cmd.Run()

	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to run command: %w", err)
		}
	}

	// Get hostname and user
	hostname, _ := os.Hostname()
	user, _ := os.LookupEnv("USER") // For Unix-like systems
	if user == "" {
		user, _ = os.LookupEnv("USERNAME") // For Windows
	}

	// Get environment variables
	envVars := make(map[string]string)
	if len(envVarsToCapture) > 0 {
		// Capture only specified environment variables
		for _, key := range envVarsToCapture {
			if val, ok := os.LookupEnv(key); ok {
				envVars[key] = val
			}
		}
	} else {
		// Capture all environment variables if envVarsToCapture is empty
		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envVars[parts[0]] = parts[1]
			}
		}
	}
	
	if len(redactPatterns) > 0 {
		var redactErr error
		envVars, redactErr = RedactEnvVars(envVars, redactPatterns)
		if redactErr != nil {
			return nil, redactErr
		}
	}

	// Calculate previous voucher hash if path is provided
	var prevVoucherHash string
	if prevVoucherPath != "" {
		hash, err := calculateFileSHA256(prevVoucherPath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate SHA256 hash of previous voucher file %s: %w", prevVoucherPath, err)
		}
		prevVoucherHash = hash
	}

	// Populate voucher
	v := &voucher.Voucher{
		MimicVersion: "1.0", // Hardcoded for now
		RecordedAt:   time.Now().UTC(),
		DurationMs:   int(duration.Milliseconds()),
		Command: voucher.Command{
			Argv: cmdArgs,
			Cwd:  getCurrentDir(),
			Env:  envVars,
		},
		ExitCode: exitCode,
		Metadata: voucher.Metadata{
			Hostname: hostname,
			User:     user,
			// TODO: Add SizeBytes and SHA256Output later
		},
		TTL:                 ttl,
		PreviousVoucherHash: prevVoucherHash,
	}

	if preserveTiming {
		v.Stdout = stdoutWriter.(*TimedChunkWriter).Chunks()
		v.Stderr = stderrWriter.(*TimedChunkWriter).Chunks()
	} else {
		// For non-preserved timing, create a single chunk
		v.Stdout = []voucher.OutputChunk{{
			DataB64: base64.StdEncoding.EncodeToString(stdoutWriter.(*bytes.Buffer).Bytes()),
		}}
		v.Stderr = []voucher.OutputChunk{{
			DataB64: base64.StdEncoding.EncodeToString(stderrWriter.(*bytes.Buffer).Bytes()),
		}}
	}

	// Write voucher to file if outputFile is provided
	if outputFile != "" {
		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal voucher to YAML: %w", err)
		}
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write voucher to file %s: %v\n", outputFile, err)
		}
		fmt.Fprintf(os.Stderr, "Voucher recorded to %s\n", outputFile)
	}

	return v, nil
}

func getCurrentDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func calculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("could not open file %s for hashing: %w", filePath, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("could not read file %s for hashing: %w", filePath, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
