# Feature Specification: Automatic Caching and Fallback

## 1. Feature Name
Automatic Caching and Fallback (`--fallback` flag)

## 2. Motivation
The core value of `mimic` in CI/CD and development is to accelerate workflows by replaying cached command behavior. Currently, when a voucher expires (due to TTL) or is missing, the `mimic replay` command simply fails. To enable true, self-maintaining caching behavior, `mimic` must be able to automatically execute a live command (the fallback) and re-record the result if the cache is stale or missing.

This feature directly addresses the "CI/CD Acceleration" use case by providing a robust, declarative caching mechanism.

## 3. Specification

### 3.1. New Flag for `mimic replay`

The `mimic replay` command will gain a new optional flag:

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--fallback <command>` | string | A shell command to execute if the voucher is missing, expired, or fails validation. The command will be executed using the system shell. |

### 3.2. Replay Logic Flow

The `mimic replay` command's execution flow will be modified as follows:

1.  **Check Voucher:** Attempt to read and validate the voucher file specified by `<voucher>`.
2.  **Voucher Valid:** If the voucher is present, not expired (if TTL is set), and passes validation (if `--validate` is used), the voucher is replayed, and the process exits with the recorded exit code.
3.  **Voucher Invalid/Missing (Fallback Triggered):** If the voucher is missing, expired, or fails validation, and the `--fallback` flag is provided:
    a.  **Execute Fallback:** The command provided to `--fallback` is executed via the system shell.
    b.  **Capture Behavior:** The execution of the fallback command is treated as a new recording. Its `stdout`, `stderr`, and `exit_code` are captured.
    c.  **Automatic Re-recording:** A new voucher file is created at the original `<voucher>` path, containing the behavior of the successful fallback command.
    d.  **Exit:** The `mimic replay` command exits with the exit code of the fallback command.
4.  **Voucher Invalid/Missing (No Fallback):** If the voucher is invalid/missing and `--fallback` is **not** provided, `mimic replay` exits with an error (current behavior).

### 3.3. Automatic Re-recording Details

When the fallback command is executed and succeeds, the new voucher must inherit the original voucher's metadata to maintain consistency:

*   **Voucher Path:** The new voucher must be written to the original path specified by the user.
*   **TTL:** The new voucher must inherit the TTL value from the *original* voucher (if it existed) or the TTL specified by the user (if a new flag is introduced for fallback TTL). For simplicity in the initial implementation, we will assume the fallback command is recorded with the same TTL as the original voucher.
*   **Signing:** If the original voucher was signed, the new voucher must also be signed using the same private key (which must be accessible via the `--private-key` flag, or a default path).

## 4. Required Code Changes

*   **`cmd/replay.go`:** Add `--fallback` flag and implement the new logic flow.
*   **`pkg/recorder/recorder.go`:** The `Record` function needs to be callable by the `replay` command's logic.
*   **`pkg/crypto/crypto.go`:** Logic for signing/verifying must be accessible for re-signing the new voucher if necessary.
