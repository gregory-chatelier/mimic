package replayer

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"gopkg.in/yaml.v3"
)

// Replay reads a voucher file and reproduces the recorded command behavior.
// It returns the exit code and an error, if any.
func Replay(voucherFile string, preserveTiming bool, speed float64) (int, error) {
	data, err := os.ReadFile(voucherFile)
	if err != nil {
		return 1, fmt.Errorf("failed to read voucher file %s: %w", voucherFile, err)
	}

	var v voucher.Voucher
	if err := yaml.Unmarshal(data, &v); err != nil {
		return 1, fmt.Errorf("failed to unmarshal voucher from %s: %w", voucherFile, err)
	}

	// Replay stdout
	for i, chunk := range v.Stdout {
		if preserveTiming {
			time.Sleep(time.Duration(float64(chunk.DelayNs) / speed))
		}
		stdout, err := base64.StdEncoding.DecodeString(chunk.DataB64)
		if err != nil {
			return 1, fmt.Errorf("failed to decode stdout chunk %d data: %w", i, err)
		}
		fmt.Print(string(stdout))
	}

	// Replay stderr
	for i, chunk := range v.Stderr {
		if preserveTiming {
			time.Sleep(time.Duration(float64(chunk.DelayNs) / speed))
		}
		stderr, err := base64.StdEncoding.DecodeString(chunk.DataB64)
		if err != nil {
			return 1, fmt.Errorf("failed to decode stderr chunk %d data: %w", i, err)
		}
		fmt.Fprint(os.Stderr, string(stderr))
	}

	return v.ExitCode, nil
}
