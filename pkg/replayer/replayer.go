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
func Replay(voucherFile string, preserveTiming bool, speed float64) error {
	data, err := os.ReadFile(voucherFile)
	if err != nil {
		return fmt.Errorf("failed to read voucher file %s: %w", voucherFile, err)
	}

	var v voucher.Voucher
	if err := yaml.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("failed to unmarshal voucher from %s: %w", voucherFile, err)
	}

	// Replay stdout
	for _, chunk := range v.Stdout {
		if preserveTiming {
			time.Sleep(time.Duration(float64(time.Duration(chunk.DelayMs)*time.Millisecond) / speed))
		}
		stdout, err := base64.StdEncoding.DecodeString(chunk.DataB64)
		if err != nil {
			return fmt.Errorf("failed to decode stdout chunk: %w", err)
		}
		fmt.Print(string(stdout))
	}

	// Replay stderr
	for _, chunk := range v.Stderr {
		if preserveTiming {
			time.Sleep(time.Duration(float64(time.Duration(chunk.DelayMs)*time.Millisecond) / speed))
		}
		stderr, err := base64.StdEncoding.DecodeString(chunk.DataB64)
		if err != nil {
			return fmt.Errorf("failed to decode stderr chunk: %w", err)
		}
		fmt.Fprint(os.Stderr, string(stderr))
	}

	// Exit with the recorded exit code
	os.Exit(v.ExitCode)

	return nil // This line is unreachable due to os.Exit
}