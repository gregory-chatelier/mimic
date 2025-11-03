# `mimic`

> Record once. Replay forever.

A deterministic, tamper-proof command behavior recorder for developers, systems and educators.

## 1. Summary

`mimic` records the **behavioral contract** of a shell command—its standard output, standard error, exit code, and environment metadata—and stores it as a **cryptographically verifiable voucher** (`.vcr` file).

Later, that voucher can be **replayed** to reproduce the command’s behavior *exactly*, without executing the original command again.

## 2. Core concepts

`mimic` is built around two core concepts:

1.  **Recording (`mimic record`):** When you record a command, `mimic` executes it, captures its entire behavioral contract, and saves it to a `.vcr` file. This file is a self-contained, human-readable (YAML) voucher.

2.  **Replaying (`mimic replay`):** When you replay a voucher, `mimic` reproduces the recorded behavior *without* running the original command. It prints the same standard output and standard error, and exits with the same exit code.

### Tamper-proof vouchers

For security and auditing, vouchers can be digitally signed using an Ed25519 key pair. This ensures that a voucher has not been tampered with since it was recorded, providing a trustworthy record of command execution.

### Why not just use shell redirection (`>`)?

Simple shell redirection (`>`) only captures **Standard Output (stdout)**. `mimic` captures the entire **Behavioral Contract** of the command, which is essential for building reliable and auditable systems.

The key difference is the ability to **reproduce failure states**. If a command fails, `mimic` records the error message and the non-zero exit code. When replayed, the consuming script behaves *exactly* as if the original command had just failed, making your tests and pipelines truly deterministic.

File redirection gives you data snapshots. `mimic` gives you **executable proofs**.

When you need to:

*   **Prove what happened**
*   **Reproduce failures reliably**
*   **Demo with confidence**
*   **Audit with cryptographic certainty**

...you need behavior recording, not only data storage.

## 4. Usage

The command structure is:

```bash
mimic [command] [flags]
```

### Commands

| Command   | Description                                                  |
| :-------- | :----------------------------------------------------------- |
| `record`  | Record a command’s behavior into a voucher.                  |
| `replay`  | Replay a recorded voucher.                                   |
| `verify`  | Verify the integrity and signature of a voucher.             |
| `inspect` | View voucher metadata and summary.                           |
| `refresh` | Refresh or update an existing voucher by re-running the command. |
| `keygen`  | Generate a signing key pair.                                 |

### Options

For a full list of flags for each command, use `mimic [command] --help`.

#### `record` Flags

| Flag                | Description                                           |
| :------------------ | :---------------------------------------------------- |
| `-o, --output`      | Output voucher file (default: auto-named).            |
| `--sign`            | Sign the voucher with a private key.                  |
| `--private-key`     | Path to the private key for signing.                  |
| `--with-env`        | Include environment variables in the recording.       |
| `--preserve-timing` | Record time intervals between outputs.                |
| `--ttl`             | Expire voucher after a specified duration (e.g., `24h`). |

#### `replay` Flags

| Flag                | Description                                                  |
| :------------------ | :----------------------------------------------------------- |
| `--validate`        | Verify signature and integrity before replay.                |
| `--public-key`      | Path to the public key for verification.                     |
| `--preserve-timing` | Simulate original timing delays.                             |
| `--speed`           | Adjust playback speed (e.g., 0.5 to slow down, 2.0 to speed up). |
| `--fallback`        | Execute real command to refresh cache if voucher is missing or invalid. |

## 5. Examples

### Example 1: Tamper-proof auditing

**Scenario:** You need to prove that a critical command (e.g., a database migration, a financial report query, or a security scan) was executed at a specific server time and produced a specific output, and that the record has not been altered.

```bash
# 1. Generate a key pair (done once)
mimic keygen

# 2. Record and sign the sensitive command
mimic record -o audit.vcr --sign --private-key mimic.key -- \
  psql -c "SELECT COUNT(*) FROM production_users;"

# 3. Verify the voucher's integrity later
mimic verify --public-key mimic.pub audit.vcr
# ✔ Signature valid (ed25519)
```

### Example 2: Education

**Scenario:** You are recording a long-running script for a tutorial or demo. You want to preserve the original timing for accuracy but be able to speed up the playback for the audience. The `--speed` flag is applied during replay.

```bash
# 1. Record the command, preserving the original 10-second delay
mimic record -o demo_build.vcr --preserve-timing -- \
  bash -c 'echo "Starting build..."; sleep 10; echo "Build complete."'

# 2. Replay the demo at 2x speed (5-second delay)
mimic replay demo_build.vcr --preserve-timing --speed 2
# Output appears with a 5-second delay.
```

### Example 3: Self-healing caching

**Scenario:** You want to cache the result of a command (e.g., downloading a large machine learning model) for 1 week. If the cache is fresh, use it instantly. If it's expired or missing, run the live command and automatically update the cache.

```bash
# The first run: Cache is missing. Fallback runs, and a new voucher is created.
mimic replay model_download.vcr --validate --fallback "wget https://ml.corp/tts-v3.zip" --ttl 1w
# Output: LIVE download-model command runs, result is saved to model_download.vcr

# Subsequent runs (within 1 week): Cache is fresh.
mimic replay model_download.vcr --validate --fallback "wget https://ml.corp/tts-v3.zip" --ttl 1w
# Output: Instant replay from model_download.vcr (Cache Hit)

# Run after 1 week: Cache is stale. Fallback runs again, and the voucher is refreshed.
mimic replay model_download.vcr --validate --fallback "wget https://ml.corp/tts-v3.zip" --ttl 1w
# Output: LIVE download-model command runs, result is saved to model_download.vcr (Cache Refresh)
```

## 6. License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
