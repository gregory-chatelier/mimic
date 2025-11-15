package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/gregory-chatelier/mimic/pkg/replayer"
	"github.com/gregory-chatelier/mimic/pkg/validation"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	validateVoucher      bool
	publicKeyPath        string
	replayPreserveTiming bool
	speed                float64
	useFallback          bool
	requireSignature     bool // New flag
)

// RunReplayCommand contains the core logic for the replay command,
// returning the exit code and an error, rather than calling os.Exit directly.
func RunReplayCommand(voucherFile string, fallbackCmdToExecute []string, validateVoucher bool, publicKeyPath string, privateKeyPath string, replayPreserveTiming bool, speed float64, useFallback bool, requireSignature bool) (int, error) {
	isCacheStale := false
	isSecurityFailure := false

	// 1. Validate speed flag
	if speed <= 0 {
		return 1, fmt.Errorf("the --speed multiplier must be greater than 0")
	}

	// 2. Validate public key path if validation is requested
	if validateVoucher || requireSignature { // If requireSignature, validation is implicitly needed
		if publicKeyPath == "" {
			return 1, fmt.Errorf("--validate or --require-signature requires a --public-key path to be specified")
		}
		if err := validation.ValidateFileExists(publicKeyPath, "Public key file"); err != nil {
			return 1, err
		}
	}

	// 3. Validate private key path if fallback re-signing is requested
	if useFallback && privateKeyPath != "" {
		if err := validation.ValidateFileExists(privateKeyPath, "Private key file"); err != nil {
			return 1, fmt.Errorf("%w for re-signing on fallback", err)
		}
	}

	// --- Check for valid voucher ---

	// Try to read the file first
	data, err := os.ReadFile(voucherFile)
	if os.IsNotExist(err) {
		isCacheStale = true
	} else if err != nil {
		return 1, fmt.Errorf("failed to read voucher file %s: %w", voucherFile, err)
	}

	var v voucher.Voucher
	if !isCacheStale {
		if err := yaml.Unmarshal(data, &v); err != nil {
			// File exists but is malformed, treat as cache stale if fallback is available
			fmt.Fprintf(os.Stderr, "Warning: Voucher file '%s' is malformed (%v). Treating as cache stale.\n", voucherFile, err)
			isCacheStale = true
		}

		// Perform validation if required
		if (validateVoucher || requireSignature) && !isCacheStale {
			// Load public key (already validated existence above)
			pk, err := crypto.LoadPublicKey(publicKeyPath)
			if err != nil {
				// This error should ideally not happen if os.Stat passed, but good to check
				return 1, fmt.Errorf("error loading public key for validation: %w", err)
			}

			// Check TTL
			if v.TTL > 0 && time.Since(v.RecordedAt) > v.TTL {
				isCacheStale = true
				fmt.Fprintf(os.Stderr, "Warning: Voucher has expired (Recorded at %s, TTL %s). Treating as cache stale.\n", v.RecordedAt.Format(time.RFC3339), v.TTL.String())
			}

			// Check signature (Security Critical Check)
			if !isCacheStale { // Only check signature if not already stale from TTL
				if v.Signature.SignatureB64 == "" {
					isSecurityFailure = true
					if requireSignature {
						return 1, fmt.Errorf("voucher is not signed, but --require-signature flag was set")
					}
					fmt.Fprintln(os.Stderr, "Warning: Voucher is not signed.")
				} else {
					signatureBytes, err := crypto.DecodeBase64(v.Signature.SignatureB64)
					if err != nil {
						return 1, fmt.Errorf("error decoding signature for validation: %w", err)
					}
					verifiableVoucher := v
					verifiableVoucher.Signature = voucher.Signature{}
					verifiableData, _ := yaml.Marshal(verifiableVoucher)

					if !crypto.VerifySignature(pk, verifiableData, signatureBytes) {
						isSecurityFailure = true
						return 1, fmt.Errorf("voucher signature is invalid! This indicates tampering")
					}
				}
			}

			if isSecurityFailure {
				return 1, fmt.Errorf("security validation failed") // Exit immediately on security failure
			}

			if !isCacheStale {
				fmt.Fprintln(os.Stderr, "Voucher validated successfully.")
			}
		}
	}

	// --- Fallback Logic ---
	if isCacheStale {
		if useFallback {
			if len(fallbackCmdToExecute) == 0 {
				return 1, fmt.Errorf("--fallback flag used, but no command provided after '--'")
			}

			// Join the command for logging purposes
			fallbackCommandStr := strings.Join(fallbackCmdToExecute, " ")
			fmt.Fprintf(os.Stderr, "Cache is stale or missing. Executing fallback command: %s\n", fallbackCommandStr)

			// 1. Prepare to record the fallback command
			tmpVCRFile, err := os.CreateTemp(os.TempDir(), "mimic-fallback-*.vcr")
			if err != nil {
				return 1, fmt.Errorf("error creating temporary file for fallback recording: %w", err)
			}
			tmpVCRFile.Close()
			defer os.Remove(tmpVCRFile.Name())

			// Determine if timing should be preserved for the fallback
			recordFallbackPreserveTiming := replayPreserveTiming // Use replay's preserve timing setting for fallback recording

			var envVarsToCapture []string
			if len(v.Command.Env) > 0 {
				envVarsToCapture = []string{} // Capture all env vars if original had them
			}

			// Record the command using the recorder package
			_, err = recorder.Record(fallbackCmdToExecute, tmpVCRFile.Name(), envVarsToCapture, v.TTL, recordFallbackPreserveTiming, []string{})

			if err != nil {
				return 1, fmt.Errorf("error recording fallback command: %w", err)
			}

			// 2. Overwrite the original voucher file with the new recording
			finalData, err := os.ReadFile(tmpVCRFile.Name())
			if err != nil {
				return 1, fmt.Errorf("error reading temporary fallback voucher: %w", err)
			}

			// Implement re-signing logic if a private key is provided
			if privateKeyPath != "" {
				// Load the newly recorded voucher from the temporary file
				var newVoucher voucher.Voucher
				if err := yaml.Unmarshal(finalData, &newVoucher); err != nil {
					return 1, fmt.Errorf("error unmarshalling temporary fallback voucher for signing: %w", err)
				}

				// Load private key (already validated existence above)
				sk, err := crypto.LoadPrivateKey(privateKeyPath)
				if err != nil {
					// This error should ideally not happen if os.Stat passed, but good to check
					return 1, fmt.Errorf("error loading private key for re-signing: %w", err)
				}

				// Sign the voucher
				signedData, err := crypto.SignVoucher(newVoucher, sk)
				if err != nil {
					return 1, fmt.Errorf("error signing fallback voucher: %w", err)
				}
				finalData = signedData
				fmt.Fprintln(os.Stderr, "Voucher successfully re-signed.")
			}

			if err := os.WriteFile(voucherFile, finalData, 0644); err != nil {
				return 1, fmt.Errorf("error writing new voucher to %s: %w", voucherFile, err)
			}
			fmt.Fprintf(os.Stderr, "Voucher cache refreshed from fallback command and saved to %s\n", voucherFile)

			// 3. Replay from the newly written voucher
			replayExitCode, err := replayer.Replay(voucherFile, replayPreserveTiming, speed)
			if err != nil {
				return 1, fmt.Errorf("error replaying newly recorded voucher: %w", err)
			}
			return replayExitCode, nil
		} else {
			// No fallback command provided, exit with an error
			originalError := "Voucher is missing or failed validation/TTL check."
			if os.IsNotExist(err) {
				originalError = fmt.Sprintf("Voucher file not found at '%s'", voucherFile)
			} else if err != nil {
				originalError = fmt.Sprintf("Voucher file '%s' is unreadable or malformed", voucherFile)
			}

			return 1, fmt.Errorf("%s", originalError)
		}
	}

	// --- Default Replay (Voucher is valid) ---
	if !isCacheStale && !useFallback {
		fmt.Fprintln(os.Stderr, "Voucher is valid. Replaying from cache.")
	} else if !isCacheStale && useFallback {
		fmt.Fprintln(os.Stderr, "Voucher is valid. Replaying from cache (Fallback ignored).")
	}

	replayExitCode, err := replayer.Replay(voucherFile, replayPreserveTiming, speed)
	if err != nil {
		return 1, fmt.Errorf("error replaying voucher: %w", err)
	}
	return replayExitCode, nil
}

var replayCmd = &cobra.Command{
	Use:   "replay [flags] <voucher> [-- <fallback command> [args...]]",
	Short: "Replay a recorded command behavior",
	Long: `The replay command reads a .vcr file, emits its recorded standard output and standard error,
and exits with the recorded exit code, reproducing the original command's behavior.

If the --fallback flag is used, the command following the '--' separator will be executed
if the voucher is missing, expired, or malformed.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("replayCmd args: %v\n", args)
		voucherFile := args[0]

		// 1. Separate mimic flags from the fallback command
		separatorIdx := -1
		for i, arg := range args {
			if arg == "--" {
				separatorIdx = i
				break
			}
		}

		var fallbackCmdToExecute []string
		if separatorIdx != -1 {
			fallbackCmdToExecute = args[separatorIdx+1:]
		}

		exitCode, err := RunReplayCommand(voucherFile, fallbackCmdToExecute, validateVoucher, publicKeyPath, privateKeyPath, replayPreserveTiming, speed, useFallback, requireSignature)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(exitCode)
	},
}

func init() {
	replayCmd.Flags().BoolVarP(&validateVoucher, "validate", "v", false, "Verify signature and integrity before replay")
	replayCmd.Flags().StringVarP(&publicKeyPath, "public-key", "p", "", "Path to the public key for verification")
	replayCmd.Flags().BoolVarP(&replayPreserveTiming, "preserve-timing", "t", false, "Simulate original timing delays")
	replayCmd.Flags().Float64VarP(&speed, "speed", "s", 1.0, "Adjust playback speed (e.g., 0.5 to slow down, 2.0 to speed up)")
	replayCmd.Flags().BoolVar(&useFallback, "fallback", false, "Execute real command to refresh cache if voucher is missing or invalid")
	replayCmd.Flags().BoolVar(&requireSignature, "require-signature", false, "Require the voucher to be signed for replay")
}
