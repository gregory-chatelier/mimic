# mimic

> Record once. Replay forever.

A deterministic, tamper-proof command behavior recorder

`mimic` records the behavior of a shell command : its standard output, standard error, exit code, and environment metadata, and stores it as a cryptographically verifiable voucher (.vcr file).

Later, that voucher can be replayed to reproduce the command’s behavior exactly, without executing the original command again.

## Core concepts

`mimic` is built around two core concepts:

1.  Recording (mimic record): When you record a command, mimic executes it, captures its entire behavioral contract, and saves it to a .vcr file. This file is a self-contained, human-readable (YAML) voucher.

2.  Replaying (mimic replay): When you replay a voucher, mimic reproduces the recorded behavior without running the original command. It prints the same standard output and standard error, and exits with the same exit code.

### Tamper-proof vouchers

For security and auditing, vouchers can be digitally signed using an Ed25519 key pair. This ensures that a voucher has not been tampered with since it was recorded, providing a trustworthy record of command execution.

### Why not just use shell redirection ( > )?

`mimic` captures the entire behavioral contract of the command, which can be use for building reliable and auditable systems.

File redirection gives you data snapshots. `mimic` gives you executable proofs.

When you need to:

*   Prove what happened
*   Reproduce failures reliably
*   Demo with confidence
*   Audit with cryptographic certainty

...you need behavior recording, not only data storage.

## Installation

This command will download and install `mimic` to a standard location for your system.

**Recommended (User-level):**
Installs to `$HOME/.local/bin` (Linux/macOS) or a user-specific `bin` directory (Windows).

```bash
curl -sSfL https://raw.githubusercontent.com/gregory-chatelier/mimic/main/install.sh | sh
```

**System-wide (Requires `sudo`):**
Installs to `/usr/local/bin`.

```bash
sudo curl -sSfL https://raw.githubusercontent.com/gregory-chatelier/mimic/main/install.sh | sh
```

**Custom Directory:**
Use the `INSTALL_DIR` environment variable to specify a custom path.

```bash
curl -sSfL https://raw.githubusercontent.com/gregory-chatelier/mimic/main/install.sh | INSTALL_DIR=$HOME/bin sh
```

## Usage

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
| `--env-vars`        | Comma-separated list of environment variables to include in the recording (e.g., `PATH,HOME`). If omitted, all environment variables are included. |
| `--redact-patterns` | Comma-separated list of regex patterns to redact sensitive information from recorded environment variables (e.g., `(?i)password=.*,API_KEY=.*`). |
| `--preserve-timing` | Record time intervals between outputs.                |
| `--ttl`             | Expire voucher after a specified duration (e.g., `24h`). |

#### `replay` Flags

| Flag                | Description                                                  |
| :------------------ | :----------------------------------------------------------- |
| `-v, --validate`    | Verify signature and integrity before replay.                |
| `-p, --public-key`  | Path to the public key for verification.                     |
| `-t, --preserve-timing` | Simulate original timing delays.                             |
| `-s, --speed`       | Adjust playback speed (e.g., 0.5 to slow down, 2.0 to speed up). |
| `--fallback`        | Execute real command to refresh cache if voucher is missing or invalid. |
| `--require-signature` | Require the voucher to be signed for replay.                 |

#### `verify` Flags

| Flag                | Description                                                  |
| :------------------ | :----------------------------------------------------------- |
| `-p, --public-key`  | Path to the public key for verification.                     |

#### `refresh` Flags

| Flag                | Description                                                  |
| :------------------ | :----------------------------------------------------------- |
| `-p, --public-key`  | Path to the public key for verifying the original voucher (if signed). |

#### `keygen` Flags

| Flag                | Description                                                  |
| :------------------ | :----------------------------------------------------------- |
| `-o, --output`      | Output directory for the generated key pair (default: current directory). |

## Handling Complex Commands (Pipes and Redirections)

A crucial question is how `mimic` handles shell operators like pipes (`|`) and redirections (`>`).

**The Golden Rule:** Always wrap complex commands in an explicit shell (like `bash -c "..."`).

**Why?** `mimic` records the `stdout` and `stderr` of the single process it is asked to execute. Shell operators like `|` and `>` are not part of a command; they are instructions for the *shell* to connect multiple commands or redirect their output.

- **Incorrect:** `mimic record -- ls | grep .md`
  - Your current shell runs `mimic record -- ls` and pipes its output to `grep`. `mimic` only records `ls`, not the final result.

- **Correct:** `mimic record -- bash -c "ls | grep .md"`
  - `mimic` records the `bash` process. The `bash` process runs the entire pipeline (`ls | grep .md`), and `mimic` captures the final, filtered output.

Using `bash -c` ensures that the entire command chain is treated as a single, atomic operation, which is exactly what you want to record for a verifiable audit.

## Examples

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

### Example 3: Self-healing caching for expensive operations

**Scenario:** Use `mimic` to cache the output of slow, expensive, or rate-limited operations in CI/CD pipelines. If the voucher is missing or expired, the live command is executed and the cache is automatically refreshed.

```bash
# Cache an 'npm audit' for 1 day
mimic replay npm-audit.vcr --fallback --ttl 1d -- npm audit
```

### Example 4: "Time Capsule" for production incidents

**The Magic:** Record production commands during incidents for post-mortem replay.

```bash
# During incident:
mimic record -o incident-2024-db-query.vcr --sign -- \
  psql -c "EXPLAIN ANALYZE SELECT * FROM slow_table"

# In post-mortem (weeks later):
mimic inspect incident-2024-db-query.vcr
mimic replay incident-2024-db-query.vcr --preserve-timing
# Relive the EXACT slow query behavior
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
