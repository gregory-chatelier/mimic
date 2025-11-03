#!/bin/bash

# This script demonstrates the usage of the mimic CLI tool.

set -e

# --- Configuration ---
APP_NAME="mimic"
DEMO_DIR="$(dirname "$0")"

# Ensure mimic is built
if [ ! -f "./$APP_NAME" ]; then
    echo "Building $APP_NAME..."
    go build -o "./$APP_NAME" .
fi

# --- Helper Functions ---
run_cmd() {
    echo "\n>>> Running: $@"
    eval "$@"
    echo "<<< Command finished."
}

# --- Demo Start ---
echo "======================================="
echo "  Mimic CLI Demo Script"
echo "======================================="

# Clean up previous demo artifacts
run_cmd "rm -f $DEMO_DIR/*.vcr $DEMO_DIR/*.key $DEMO_DIR/*.pub"

# 1. Generate a key pair
echo "\n--- 1. Generating Key Pair ---"
run_cmd "./$APP_NAME keygen -o $DEMO_DIR"

# 2. Record a simple command without timing or signing
echo "\n--- 2. Recording a simple command (no timing, no signing) ---"
run_cmd "./$APP_NAME record -o $DEMO_DIR/echo_hello.vcr -- echo 'Hello from mimic!'"

# 3. Inspect the recorded voucher
echo "\n--- 3. Inspecting the echo_hello.vcr voucher ---"
run_cmd "./$APP_NAME inspect $DEMO_DIR/echo_hello.vcr"

# 4. Replay the simple command
echo "\n--- 4. Replaying echo_hello.vcr ---"
run_cmd "./$APP_NAME replay $DEMO_DIR/echo_hello.vcr"

# 5. Record a command with timing and signing
echo "\n--- 5. Recording a command with timing and signing ---"
run_cmd "./$APP_NAME record -o $DEMO_DIR/timed_output.vcr --sign --private-key $DEMO_DIR/mimic.key --preserve-timing -- bash -c 'echo \"First line\"; sleep 0.1; echo \"Second line\"; sleep 0.2; echo \"Third line\"'"

# 6. Inspect the timed and signed voucher
echo "\n--- 6. Inspecting the timed_output.vcr voucher ---"
run_cmd "./$APP_NAME inspect $DEMO_DIR/timed_output.vcr"

# 7. Replay the timed command with preserved timing
echo "\n--- 7. Replaying timed_output.vcr with preserved timing ---"
run_cmd "./$APP_NAME replay --preserve-timing $DEMO_DIR/timed_output.vcr"

# 8. Replay the timed command at 2x speed
echo "\n--- 8. Replaying timed_output.vcr at 2x speed ---"
run_cmd "./$APP_NAME replay --preserve-timing --speed 2 $DEMO_DIR/timed_output.vcr"

# 9. Verify the signed voucher
echo "\n--- 9. Verifying timed_output.vcr ---"
run_cmd "./$APP_NAME verify --public-key $DEMO_DIR/mimic.pub $DEMO_DIR/timed_output.vcr"

# 10. Record a command with TTL and refresh it
echo "\n--- 10. Recording a command with a short TTL (1s) ---"
run_cmd "./$APP_NAME record -o $DEMO_DIR/short_ttl.vcr --ttl 1s -- echo 'This voucher will expire soon'"
run_cmd "./$APP_NAME inspect $DEMO_DIR/short_ttl.vcr"

echo "\n--- 10b. Waiting for 2 seconds for the voucher to expire ---"
run_cmd "sleep 2"

echo "\n--- 10c. Attempting to replay expired voucher (should fail with --validate) ---"
run_cmd "./$APP_NAME replay --validate --public-key $DEMO_DIR/mimic.pub $DEMO_DIR/short_ttl.vcr || echo 'Replay of expired voucher failed as expected.'"

echo "\n--- 10d. Refreshing the expired voucher ---"
run_cmd "./$APP_NAME refresh $DEMO_DIR/short_ttl.vcr"
run_cmd "./$APP_NAME inspect $DEMO_DIR/short_ttl.vcr"

# 11. Record a command with environment variables
echo "\n--- 11. Recording a command with environment variables ---"
run_cmd "./$APP_NAME record -o $DEMO_DIR/env_test.vcr --with-env -- bash -c 'echo \"My custom var: $MY_CUSTOM_VAR\"'"
run_cmd "./$APP_NAME inspect $DEMO_DIR/env_test.vcr"

echo "\n--- 11b. Replaying the command with environment variables ---"
run_cmd "MY_CUSTOM_VAR='Hello from outside!' ./$APP_NAME replay $DEMO_DIR/env_test.vcr"

# 12. Automatic Caching and Fallback (The High-Value Feature)
echo "\n--- 12. Automatic Caching and Fallback ---"

# Clean up the specific voucher file first
run_cmd "rm -f $DEMO_DIR/cache_test.vcr"

# 12a. First run: Cache miss, executes fallback, records new voucher
echo "\n--- 12a. First run: Cache Miss (Voucher missing) ---"
run_cmd "./$APP_NAME replay $DEMO_DIR/cache_test.vcr --fallback \"echo 'LIVE RUN: Cache was missing, now recorded.'\""
run_cmd "./$APP_NAME inspect $DEMO_DIR/cache_test.vcr"

# 12b. Second run: Cache hit, replays instantly
echo "\n--- 12b. Second run: Cache Hit (Replays instantly) ---"
run_cmd "./$APP_NAME replay $DEMO_DIR/cache_test.vcr --fallback \"echo 'LIVE RUN: Cache was missing, now recorded.'\""

# 12c. Third run: Cache stale (TTL expired), executes fallback, refreshes cache
echo "\n--- 12c. Third run: Cache Stale (TTL expired) ---"
run_cmd "./$APP_NAME record -o $DEMO_DIR/stale_cache.vcr --ttl 1s -- echo 'STALE: This is the old cache.'"
run_cmd "sleep 2"
run_cmd "./$APP_NAME replay $DEMO_DIR/stale_cache.vcr --validate --fallback \"echo 'LIVE RUN: Cache was stale, now refreshed.'\""
run_cmd "./$APP_NAME inspect $DEMO_DIR/stale_cache.vcr"

echo "======================================="
echo "  Demo Script Finished!"
echo "======================================="
