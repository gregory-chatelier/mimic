package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
	"github.com/gregory-chatelier/mimic/pkg/recorder"
	"github.com/gregory-chatelier/mimic/pkg/replayer"
	"github.com/gregory-chatelier/mimic/pkg/voucher"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	validateVoucher      bool
	publicKeyPath        string
	replayPreserveTiming bool
	speed                float64
	fallbackCommand      string
)

var replayCmd = &cobra.Command{
	Use:   "replay [flags] <voucher>",
	Short: "Replay a recorded command behavior",
	Long: `The replay command reads a .vcr file, emits its recorded standard output and standard error,
and exits with the recorded exit code, reproducing the original command's behavior.`, 
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		voucherFile := args[0]
		isCacheStale := false
		isSecurityFailure := false

		// 1. Validate speed flag
		if speed <= 0 {
			fmt.Fprintf(os.Stderr, "Error: The --speed multiplier must be greater than 0.\n")
			os.Exit(1)
		}

		// --- Check for valid voucher ---

		// Try to read the file first
		data, err := os.ReadFile(voucherFile)
		if os.IsNotExist(err) {
			isCacheStale = true
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to read voucher file %s: %v\n", voucherFile, err)
			os.Exit(1)
		}

		var v voucher.Voucher
		if !isCacheStale {
			if err := yaml.Unmarshal(data, &v); err != nil {
				// File exists but is malformed, treat as cache stale if fallback is available
				fmt.Fprintf(os.Stderr, "Warning: Voucher file is malformed (%v). Treating as cache stale.\n", err)
				isCacheStale = true
			}

			// Perform validation if required
			if validateVoucher && !isCacheStale {
				// Load public key
				pk, err := crypto.LoadPublicKey(publicKeyPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading public key for validation: %v\n", err)
					os.Exit(1)
				}

				// Check TTL
				if v.TTL > 0 && time.Since(v.RecordedAt) > v.TTL {
					isCacheStale = true
					fmt.Fprintf(os.Stderr, "Warning: Voucher has expired (Recorded at %s, TTL %s). Treating as cache stale.\n", v.RecordedAt.Format(time.RFC3339), v.TTL.String())
				}

				// Check signature (Security Critical Check)
				if !isCacheStale {
					if v.Signature.SignatureB64 == "" {
						isSecurityFailure = true
						fmt.Fprintf(os.Stderr, "Error: Voucher is not signed, but validation was requested. Failing.\n")
					} else {
						signatureBytes, err := crypto.DecodeBase64(v.Signature.SignatureB64)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error decoding signature for validation: %v\n", err)
							os.Exit(1)
						}
						verifiableVoucher := v
						verifiableVoucher.Signature = voucher.Signature{}
						verifiableData, _ := yaml.Marshal(verifiableVoucher)

						if !crypto.VerifySignature(pk, verifiableData, signatureBytes) {
							isSecurityFailure = true
							fmt.Fprintf(os.Stderr, "Error: Voucher signature is invalid! This indicates tampering. Failing.\n")
						}
					}
				}

				if isSecurityFailure {
					os.Exit(1) // Exit immediately on security failure
				}
				
				if !isCacheStale {
					fmt.Println("Voucher validated successfully.")
				}
			}
		}

		// --- Fallback Logic ---
		if isCacheStale {
			if fallbackCommand != "" {
				fmt.Printf("Cache is stale or missing. Executing fallback command: %s\n", fallbackCommand)

				// 1. Prepare to record the fallback command
				tmpVCRFile, err := os.CreateTemp(os.TempDir(), "mimic-fallback-*.vcr")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error creating temporary file for fallback recording: %v\n", err)
					os.Exit(1)
				}
				tmpVCRFile.Close()
				defer os.Remove(tmpVCRFile.Name())

				// Split the fallback command into arguments for the external shell
				shell := "sh"
				shellArg := "-c"
				if os.Getenv("SHELL") == "" && strings.Contains(os.Getenv("OS"), "Windows") {
					shell = "powershell.exe"
					shellArg = "-Command"
				}
				
				// Record the command using the recorder package
				cmdArgs := []string{shell, shellArg, fallbackCommand}
				_, err = recorder.Record(cmdArgs, tmpVCRFile.Name(), v.Command.Env != nil, v.TTL, replayPreserveTiming, "")
				
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error recording fallback command: %v\n", err)
					os.Exit(1)
				}
				
				// 2. Overwrite the original voucher file with the new recording
				finalData, err := os.ReadFile(tmpVCRFile.Name())
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading temporary fallback voucher: %v\n", err)
					os.Exit(1)
				}

				// TODO: Implement re-signing logic if required. This requires a dedicated --private-key flag on replay.
				
				if err := os.WriteFile(voucherFile, finalData, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing new voucher to %s: %v\n", voucherFile, err)
					os.Exit(1)
				}
				fmt.Printf("Voucher cache refreshed from fallback command and saved to %s.\n", voucherFile)
				
				// 3. Replay from the newly written voucher
				err = replayer.Replay(voucherFile, replayPreserveTiming, speed)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error replaying newly recorded voucher: %v\n", err)
					os.Exit(1)
				}
				// os.Exit called inside Replay

			} else {
				// No fallback command provided, exit with an error
				originalError := "Voucher is missing or failed validation/TTL check."
				if os.IsNotExist(err) {
					originalError = fmt.Sprintf("Voucher file not found at %s.", voucherFile)
				}


				fmt.Fprintf(os.Stderr, "Error: %s\n", originalError)
				os.Exit(1)
			}
		}

		// --- Default Replay (Voucher is valid) ---
		if !isCacheStale && fallbackCommand == "" {
			fmt.Println("Voucher is valid. Replaying from cache.")
		} else if !isCacheStale && fallbackCommand != "" {
			fmt.Println("Voucher is valid. Replaying from cache (Fallback ignored).")
		}

		err = replayer.Replay(voucherFile, replayPreserveTiming, speed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error replaying voucher: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(replayCmd)

	replayCmd.Flags().BoolVar(&validateVoucher, "validate", false, "Verify signature and integrity before replay")
	replayCmd.Flags().StringVar(&publicKeyPath, "public-key", "mimic.pub", "Path to the public key file for verification")
	replayCmd.Flags().BoolVar(&replayPreserveTiming, "preserve-timing", false, "Simulate original timing delays")
	replayCmd.Flags().Float64Var(&speed, "speed", 1.0, "Adjust playback speed (e.g., 2x, 0.5x)")
	replayCmd.Flags().StringVar(&fallbackCommand, "fallback", "", "Execute real command to refresh cache if voucher is missing or invalid")
}