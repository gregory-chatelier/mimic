# `mimic`

> Record once. Replay forever.

A deterministic, tamper-proof command behavior recorder for developers, CI systems and educators.

## 1. Summary

`mimic` records the **behavioral contract** of a shell command—its standard output, standard error, exit code, and environment metadata—and stores it as a **cryptographically verifiable voucher** (`.vcr` file).

Later, that voucher can be **replayed** to reproduce the command’s behavior *exactly*, without executing the original command again.

This enables:

*   **Deterministic Testing:** Eliminate flaky tests that rely on external services.
*   **Offline Development:** Work with APIs and external commands without an internet connection.
*   **Reproducible Debugging:** Capture the exact behavior of a failing command for later analysis.
*   **Tamper-Proof Auditing:** Create a verifiable audit trail of command executions.
*   **CI/CD Acceleration:** Speed up pipelines by replaying long-running commands instead of re-executing them.

## 2. Core Concepts

`mimic` is built around two core concepts:

1.  **Recording (`mimic record`):** When you record a command, `mimic` executes it, captures its entire behavioral contract, and saves it to a `.vcr` file. This file is a self-contained, human-readable (YAML) voucher.

2.  **Replaying (`mimic replay`):** When you replay a voucher, `mimic` reproduces the recorded behavior *without* running the original command. It prints the same standard output and standard error, and exits with the same exit code.

### Tamper-Proof Vouchers

For security and auditing, vouchers can be digitally signed using an Ed25519 key pair. This ensures that a voucher has not been tampered with since it was recorded, providing a trustworthy record of command execution.

## 3. Usage

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

| Flag                | Description                                      |
| :------------------ | :----------------------------------------------- |
| `--validate`        | Verify signature and integrity before replay.    |
| `--public-key`      | Path to the public key for verification.         |
| `--preserve-timing` | Simulate original timing delays.                 |
| `--speed`           | Adjust playback speed (e.g., `2.0` for 2x speed). |

## 4. Installation

This single command will download and install `mimic` to a sensible default location for your system.

**User-level Installation (Recommended for most users):**
Installs `mimic` to `$HOME/.local/bin` (Linux/macOS) or a user-specific `bin` directory (Windows).

```bash
curl -sSfL https://raw.githubusercontent.com/gregory-chatelier/mimic/main/install.sh | sh
```

**System-wide Installation (Requires `sudo`):**
Installs `mimic` to `/usr/local/bin` (Linux/macOS).

```bash
sudo sh -c "$(curl -sSfL https://raw.githubusercontent.com/gregory-chatelier/mimic/main/install.sh)"
```

**Custom Installation Directory:**

You can specify a custom installation directory using the `INSTALL_DIR` environment variable:

```bash
curl -sSfL https://raw.githubusercontent.com/gregory-chatelier/mimic/main/install.sh | INSTALL_DIR=$HOME/bin sh
```

## 5. Examples

### Example 1: Offline API Development

Record an API call once, then replay it offline as many times as you need.

```bash
# Record the API call
mimic record -o users.vcr -- curl -s https://api.github.com/users/octocat

# Replay it offline
mimic replay users.vcr | jq '.login'
# Outputs "octocat" even when offline
```

### Example 2: Deterministic CI Tests

Replace flaky integration tests with deterministic replays. Store the `.vcr` file in your repository.

```bash
# In your test setup, record the expected API behavior
mimic record -o tests/api-fixture.vcr -- npm run test:api

# In your CI pipeline, replay the fixture instead of hitting a live API
mimic replay tests/api-fixture.vcr
```

### Example 3: Debugging a Flaky Script

Capture the exact output of a flaky script to reproduce the failure reliably.

```bash
# Record the flaky script
mimic record -o flaky-run.vcr -- ./flaky_script.sh

# Replay the exact failure conditions for debugging
mimic replay flaky-run.vcr > debug.log
```

### Example 4: Tamper-Proof Compliance Record

Create a verifiable audit trail for compliance purposes.

```bash
# Generate a key pair
mimic keygen

# Record and sign a sensitive command
mimic record -o audit.vcr --sign --private-key mimic.key -- psql -c "SELECT * FROM users;"

# Verify the voucher's integrity later
mimic verify --public-key mimic.pub audit.vcr
# ✔ Signature valid (ed25519)
```

## 6. License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
