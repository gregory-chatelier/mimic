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
	requireSignature     bool
	fallbackTTL          string // TTL for voucher created by fallback
	fallbackSign         bool   // Whether to sign the voucher created by fallback
)

// RunReplayCommand contains the core logic for the replay command,
// returning the exit code and an error, rather than calling os.Exit directly.
func RunReplayCommand(voucherFile string, fallbackCmdToExecute []string, validateVoucher bool, publicKeyPath string, privateKeyPath string, useFallback bool, requireSignature bool, fallbackTTL string, fallbackSign bool) (int, error) {
	if err := validateReplayFlags(speed, validateVoucher, requireSignature, publicKeyPath, useFallback, privateKeyPath, fallbackSign); err != nil {
		return 1, err
	}

	v, err := loadVoucher(voucherFile)
	isCacheStale := err != nil
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: Voucher file '%s' is malformed (%v). Treating as cache stale.\n", voucherFile, err)
	}

	if (validateVoucher || requireSignature) && !isCacheStale {
		if err := crypto.VerifyVoucherIntegrity(v, publicKeyPath); err != nil {
			isCacheStale = true
			fmt.Fprintf(os.Stderr, "Warning: validation failed (%v). Treating as cache stale.\n", err)
		}
	}

	if isCacheStale {
		if useFallback {
			return handleFallback(voucherFile, fallbackCmdToExecute, privateKeyPath, v, fallbackTTL, fallbackSign)
		}
		return 1, fmt.Errorf("voucher is missing, malformed, or expired, and no fallback was provided")
	}

	return replayer.Replay(voucherFile, replayPreserveTiming, speed)
}

func validateReplayFlags(speed float64, validateVoucher bool, requireSignature bool, publicKeyPath string, useFallback bool, privateKeyPath string, fallbackSign bool) error {
	if speed <= 0 {
		return fmt.Errorf("the --speed multiplier must be greater than 0")
	}
	if validateVoucher || requireSignature {
		if publicKeyPath == "" {
			return fmt.Errorf("--validate or --require-signature requires a --public-key path to be specified")
		}
		if err := validation.ValidateFileExists(publicKeyPath, "Public key file"); err != nil {
			return err
		}
	}
	// Only require private key if fallback AND fallbackSign are enabled
	if useFallback && fallbackSign && privateKeyPath != "" {
		if err := validation.ValidateFileExists(privateKeyPath, "Private key file"); err != nil {
			return fmt.Errorf("%w for re-signing on fallback", err)
		}
	}
	return nil
}

func loadVoucher(voucherFile string) (*voucher.Voucher, error) {
	data, err := os.ReadFile(voucherFile)
	if err != nil {
		return nil, err
	}
	var v voucher.Voucher
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal voucher from %s: %w", voucherFile, err)
	}
	return &v, nil
}

func handleFallback(voucherFile string, fallbackCmdToExecute []string, privateKeyPath string, v *voucher.Voucher, fallbackTTL string, fallbackSign bool) (int, error) {
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
	if v != nil && len(v.Command.Env) > 0 {
		envVarsToCapture = []string{} // Capture all env vars if original had them
	}

	// Record the command using the recorder package
	var ttl time.Duration
	if fallbackTTL != "" {
		// Use --ttl flag from replay if provided
		d, err := parseDurationWithDays(fallbackTTL)
		if err != nil {
			return 1, fmt.Errorf("parsing fallback TTL duration: %w", err)
		}
		ttl = d
	} else if v != nil {
		// Otherwise use TTL from existing voucher
		ttl = v.TTL
	}
	_, err = recorder.Record(version, strings.Join(fallbackCmdToExecute, " "), fallbackCmdToExecute, tmpVCRFile.Name(), envVarsToCapture, ttl, recordFallbackPreserveTiming, []string{})
	if err != nil {
		return 1, fmt.Errorf("error recording fallback command: %w", err)
	}

	// 2. Overwrite the original voucher file with the new recording
	finalData, err := os.ReadFile(tmpVCRFile.Name())
	if err != nil {
		return 1, fmt.Errorf("error reading temporary fallback voucher: %w", err)
	}

	// Implement re-signing logic only if a private key is provided AND fallbackSign is enabled
	if fallbackSign && privateKeyPath != "" {
		// Load the newly recorded voucher from the temporary file
		var newVoucher voucher.Voucher
		if err := yaml.Unmarshal(finalData, &newVoucher); err != nil {
			return 1, fmt.Errorf("error unmarshalling temporary fallback voucher for signing: %w", err)
		}

		// Load private key (already validated existence above)
		sk, err := crypto.LoadPrivateKey(privateKeyPath)
		if err != nil {
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
	return replayer.Replay(voucherFile, replayPreserveTiming, speed)
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
		voucherFile := args[0]

		// Use Cobra's native ArgsLenAtDash() to properly handle the '--' separator.
		// This correctly identifies where the '--' separator appears, if at all.
		dashIdx := cmd.ArgsLenAtDash()
		var fallbackCmdToExecute []string

		if useFallback && dashIdx >= 0 && dashIdx < len(args) {
			// Args after '--' are the fallback command
			fallbackCmdToExecute = args[dashIdx:]
		}

		exitCode, err := RunReplayCommand(voucherFile, fallbackCmdToExecute, validateVoucher, publicKeyPath, privateKeyPath, useFallback, requireSignature, fallbackTTL, fallbackSign)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(exitCode)
	},
}

func init() {
	replayCmd.Flags().BoolVarP(&validateVoucher, "validate", "v", false, "Verify signature and integrity before replay")
	replayCmd.Flags().StringVarP(&publicKeyPath, "public-key", "p", "", "Path to the public key for verification")
	replayCmd.Flags().StringVar(&privateKeyPath, "private-key", "", "Path to the private key for re-signing on fallback (only used with --sign)")
	replayCmd.Flags().BoolVarP(&replayPreserveTiming, "preserve-timing", "t", false, "Simulate original timing delays")
	replayCmd.Flags().Float64VarP(&speed, "speed", "s", 1.0, "Adjust playback speed (e.g., 0.5 to slow down, 2.0 to speed up)")
	replayCmd.Flags().BoolVar(&useFallback, "fallback", false, "Execute real command to refresh cache if voucher is missing or invalid")
	replayCmd.Flags().BoolVar(&fallbackSign, "sign", false, "Sign the voucher created by fallback with the private key")
	replayCmd.Flags().BoolVar(&requireSignature, "require-signature", false, "Require the voucher to be signed for replay")
	replayCmd.Flags().StringVar(&fallbackTTL, "ttl", "", "TTL for voucher created by fallback (e.g., '24h', '1d'). If omitted, voucher never expires.")
}
