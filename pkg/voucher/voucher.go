package voucher

import (
	"time"
)

// OutputChunk represents a timed chunk of output.
type OutputChunk struct {
	DelayMs int    `yaml:"delay_ms"`
	DataB64 string `yaml:"data_b64"`
}

// Command represents the executed command details.
type Command struct {
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
	Algorithm    string `yaml:"algorithm,omitempty"`
	KeyID        string `yaml:"key_id,omitempty"`
	SignatureB64 string `yaml:"signature_b64,omitempty"`
}

// Voucher represents the recorded behavior of a command.
type Voucher struct {
	PreviousVoucherHash string        `yaml:"previous_voucher_hash,omitempty"`
	TTL                 time.Duration `yaml:"ttl,omitempty"`
	MimicVersion        string        `yaml:"mimic_version"`
	RecordedAt          time.Time     `yaml:"recorded_at"`
	DurationMs          int           `yaml:"duration_ms"`
	Command             Command       `yaml:"command"`
	Stdout              []OutputChunk `yaml:"stdout,omitempty"`
	Stderr              []OutputChunk `yaml:"stderr,omitempty"`
	ExitCode            int           `yaml:"exit_code"`
	Metadata            Metadata      `yaml:"metadata,omitempty"`
	Signature           Signature     `yaml:"signature,omitempty"`
}
