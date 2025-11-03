package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/gregory-chatelier/mimic/pkg/crypto"
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
)

var replayCmd = &cobra.Command{
	Use:   "replay [flags] <voucher>",
	Short: "Replay a recorded command behavior",
	Long: `The replay command reads a .vcr file, emits its recorded standard output and standard error,
and exits with the recorded exit code, reproducing the original command's behavior.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		voucherFile := args[0]

		// Input Validation
		// 1. Validate voucher file existence
		if _, err := os.Stat(voucherFile); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Voucher file not found at %s\n", voucherFile)
			os.Exit(1)
		}

		// 2. Validate speed flag
		if speed <= 0 {
			fmt.Fprintf(os.Stderr, "Error: The --speed multiplier must be greater than 0.\n")
			os.Exit(1)
		}

		if validateVoucher {
			// Load public key
			pk, err := crypto.LoadPublicKey(publicKeyPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading public key for validation: %v\n", err)
				os.Exit(1)
			}

			// Read voucher file
			data, err := os.ReadFile(voucherFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read voucher file %s: %v\n", voucherFile, err)
				os.Exit(1)
			}

			var v voucher.Voucher
			if err := yaml.Unmarshal(data, &v); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to unmarshal voucher from %s: %v\n", voucherFile, err)
				os.Exit(1)
			}

			// Validate signature
			if v.Signature.SignatureB64 == "" {
				fmt.Fprintf(os.Stderr, "Error: Voucher is not signed, cannot validate.\n")
				os.Exit(1)
			}

			signatureBytes, err := crypto.DecodeBase64(v.Signature.SignatureB64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding signature for validation: %v\n", err)
				os.Exit(1)
			}

			verifiableVoucher := v
			verifiableVoucher.Signature = voucher.Signature{}
			verifiableData, err := yaml.Marshal(verifiableVoucher)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error marshalling voucher for verification: %v\n", err)
				os.Exit(1)
			}

			if !crypto.VerifySignature(pk, verifiableData, signatureBytes) {
				fmt.Fprintf(os.Stderr, "Error: Voucher signature is invalid!\n")
				os.Exit(1)
			}

			// Validate TTL
			if v.TTL > 0 && time.Since(v.RecordedAt) > v.TTL {
				fmt.Fprintf(os.Stderr, "Error: Voucher has expired! Recorded at %s, TTL %s.\n", v.RecordedAt.Format(time.RFC3339), v.TTL.String())
				os.Exit(1)
			}
			fmt.Println("Voucher validated successfully.")
		}

		err := replayer.Replay(voucherFile, replayPreserveTiming, speed)
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
	replayCmd.Flags().BoolVar(&preserveTiming, "preserve-timing", false, "Simulate original timing delays")
	replayCmd.Flags().Float64Var(&speed, "speed", 1.0, "Adjust playback speed (e.g., 2x, 0.5x)")
	// replayCmd.Flags().StringVar(&fallbackCmd, "fallback", "", "Execute real command if voucher missing or invalid")
}
