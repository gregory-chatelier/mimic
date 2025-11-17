package voucher

import (
	"encoding/base64"
	"time"
)

// Duration is a wrapper for time.Duration to ensure consistent marshalling.
type Duration time.Duration

// Timestamp is a wrapper for time.Time to ensure consistent marshalling.
type Timestamp time.Time

// OutputChunk represents a timed chunk of output.
type OutputChunk struct {
	DelayNs int64  `yaml:"delay_ns"`
	DataB64 string `yaml:"data_b64"`
}

// Command represents the executed command details.
type Command struct {
	Raw  string            `yaml:"raw"` // The exact command string as provided by the user
	Argv []string          `yaml:"argv"`
	Cwd  string            `yaml:"cwd"`
	Env  map[string]string `yaml:"env,omitempty"`
}

// Metadata represents additional information about the recording.
type Metadata struct {
	Hostname     string `yaml:"hostname,omitempty"`
	User         string `yaml:"user,omitempty"`
	SizeBytes    int    `yaml:"size_bytes,omitempty"`
	SHA256Output string `yaml:"sha256_output,omitempty"`
}

// Signature represents the cryptographic signature of the voucher.
type Signature struct {
	Algorithm      string `yaml:"algorithm,omitempty"`
	KeyID          string `yaml:"key_id,omitempty"`
	SignatureB64   string `yaml:"signature_b64,omitempty"`
	ChecksumSHA256 string `yaml:"checksum_sha256,omitempty"`
}

// Voucher represents the recorded behavior of a command.
type Voucher struct {
	PreviousVoucherHash string        `yaml:"previous_voucher_hash,omitempty"`
	TTL                 time.Duration `yaml:"ttl,omitempty"`
	MimicVersion        string        `yaml:"mimic_version"`
	RecordedAt          time.Time     `yaml:"recorded_at"`
	DurationNs          int64         `yaml:"duration_ns"`
	Command             Command       `yaml:"command"`
	Stdout              []OutputChunk `yaml:"stdout,omitempty"`
	Stderr              []OutputChunk `yaml:"stderr,omitempty"`
	ExitCode            int           `yaml:"exit_code"`
	Metadata            Metadata      `yaml:"metadata,omitempty"`
	Signature           Signature     `yaml:"signature,omitempty"`
	PreserveTiming      bool          `yaml:"preserve_timing,omitempty"` // New field
}

// GetDecodedStdout is a helper method on the voucher for tests
func (v *Voucher) GetDecodedStdout() ([]byte, error) {
	var stdout []byte
	for _, chunk := range v.Stdout {
		decoded, err := DecodeChunkData(chunk.DataB64)
		if err != nil {
			return nil, err
		}
		stdout = append(stdout, decoded...)
	}
	return stdout, nil
}

// DecodeChunkData is a helper in the voucher package for tests
func DecodeChunkData(dataB64 string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(dataB64)
}
