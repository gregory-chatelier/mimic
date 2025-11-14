#!/bin/bash

# This script demonstrates the usage of the mimic CLI tool.

set -e

# --- Configuration ---
APP_NAME="./mimic"
DEMO_DIR="$(dirname "$0")"
AUTO_DEMO="false" # Set to "true" to run the demo automatically without key presses

# --- Helper Functions ---
wait_for_keypress() {
    if [ "$AUTO_DEMO" != "true" ]; then
        echo -n "Press any key to continue..."
        read -n 1 -s
        echo ""
    fi
}

# --- Demo Start ---
echo "======================================="
echo "  Mimic CLI Demo Script"
echo "======================================="

# Clean up previous demo artifacts
echo "\n>>> Running: rm -f $DEMO_DIR/*.vcr $DEMO_DIR/*.key $DEMO_DIR/*.pub"
rm -f "$DEMO_DIR"/*.vcr "$DEMO_DIR"/*.key "$DEMO_DIR"/*.pub
echo "<<< Command finished."

wait_for_keypress

# 1. Generate a key pair
echo "\n--- 1. Generating Key Pair ---"
echo "\n>>> Running: $APP_NAME keygen -o $DEMO_DIR"
"$APP_NAME" keygen -o "$DEMO_DIR"
echo "<<< Command finished."

wait_for_keypress

# 2. Record a simple command without timing or signing
echo "\n--- 2. Recording a simple command (no timing, no signing) ---"
echo "\n>>> Running: $APP_NAME record -o $DEMO_DIR/echo_hello.vcr -- echo 'Hello from mimic!'"
"$APP_NAME" record -o "$DEMO_DIR/echo_hello.vcr" -- echo 'Hello from mimic!'
echo "<<< Command finished."

wait_for_keypress

# 3. Inspect the recorded voucher
echo "\n--- 3. Inspecting the echo_hello.vcr voucher ---"
echo "\n>>> Running: $APP_NAME inspect $DEMO_DIR/echo_hello.vcr"
"$APP_NAME" inspect "$DEMO_DIR/echo_hello.vcr"
echo "<<< Command finished."

wait_for_keypress

# 4. Replay the simple command
echo "\n--- 4. Replaying echo_hello.vcr ---"
echo "\n>>> Running: $APP_NAME replay $DEMO_DIR/echo_hello.vcr"
"$APP_NAME" replay "$DEMO_DIR/echo_hello.vcr"
echo "<<< Command finished."

wait_for_keypress

# 5. Record a command with timing and signing
echo "\n--- 5. Recording a command with timing and signing ---"
echo "\n>>> Running: $APP_NAME record -o $DEMO_DIR/timed_output.vcr --sign --private-key $DEMO_DIR/mimic.key --preserve-timing -- bash -c 'echo \"First line\"; sleep 0.1; echo \"Second line\"; sleep 0.2; echo \"Third line\"'"
"$APP_NAME" record -o "$DEMO_DIR/timed_output.vcr" --sign --private-key "$DEMO_DIR/mimic.key" --preserve-timing -- bash -c 'echo "First line"; sleep 0.1; echo "Second line"; sleep 0.2; echo "Third line"'
echo "<<< Command finished."

wait_for_keypress

# 6. Inspect the timed and signed voucher
echo "\n--- 6. Inspecting the timed_output.vcr voucher ---"
echo "\n>>> Running: $APP_NAME inspect $DEMO_DIR/timed_output.vcr"
"$APP_NAME" inspect "$DEMO_DIR/timed_output.vcr"
echo "<<< Command finished."

wait_for_keypress

# 7. Replay the timed command with preserved timing
echo "\n--- 7. Replaying timed_output.vcr with preserved timing ---"
echo "\n>>> Running: $APP_NAME replay --preserve-timing $DEMO_DIR/timed_output.vcr"
"$APP_NAME" replay --preserve-timing "$DEMO_DIR/timed_output.vcr"
echo "<<< Command finished."

wait_for_keypress

# 8. Replay the timed command at 2x speed
echo "\n--- 8. Replaying timed_output.vcr at 2x speed ---"
echo "\n>>> Running: $APP_NAME replay --preserve-timing --speed 2 $DEMO_DIR/timed_output.vcr"
"$APP_NAME" replay --preserve-timing --speed 2 "$DEMO_DIR/timed_output.vcr"
echo "<<< Command finished."

wait_for_keypress

# 9. Verify the signed voucher
echo "\n--- 9. Verifying timed_output.vcr ---"
echo "\n>>> Running: ./$APP_NAME verify --public-key $DEMO_DIR/mimic.pub $DEMO_DIR/timed_output.vcr"
"$APP_NAME" verify --public-key "$DEMO_DIR/mimic.pub" "$DEMO_DIR/timed_output.vcr"
echo "<<< Command finished."

wait_for_keypress

# 10. Record a command with TTL and refresh it
echo "\n--- 10. Recording a command with a short TTL (1s) ---"
echo "\n>>> Running: ./$APP_NAME record -o $DEMO_DIR/short_ttl.vcr --ttl 1s -- echo 'This voucher will expire soon'"
"$APP_NAME" record -o "$DEMO_DIR/short_ttl.vcr" --ttl 1s -- echo 'This voucher will expire soon'
echo "<<< Command finished."
echo "\n>>> Running: ./$APP_NAME inspect $DEMO_DIR/short_ttl.vcr"
"$APP_NAME" inspect "$DEMO_DIR/short_ttl.vcr"
echo "<<< Command finished."

wait_for_keypress

echo "\n--- 10b. Waiting for 2 seconds for the voucher to expire ---"
echo "\n>>> Running: sleep 2"
sleep 2
echo "<<< Command finished."

wait_for_keypress

echo "\n--- 10c. Attempting to replay expired voucher (should fail with --validate) ---"
echo "\n>>> Running: ./$APP_NAME replay --validate --public-key $DEMO_DIR/mimic.pub $DEMO_DIR/short_ttl.vcr || echo 'Replay of expired voucher failed as expected.'"
"$APP_NAME" replay --validate --public-key "$DEMO_DIR/mimic.pub" "$DEMO_DIR/short_ttl.vcr" || echo 'Replay of expired voucher failed as expected.'
echo "<<< Command finished."

wait_for_keypress

echo "\n--- 10d. Refreshing the expired voucher ---"
echo "\n>>> Running: ./$APP_NAME refresh $DEMO_DIR/short_ttl.vcr"
"$APP_NAME" refresh "$DEMO_DIR/short_ttl.vcr"
echo "<<< Command finished."
echo "\n>>> Running: ./$APP_NAME inspect $DEMO_DIR/short_ttl.vcr"
"$APP_NAME" inspect "$DEMO_DIR/short_ttl.vcr"
echo "<<< Command finished."

wait_for_keypress

# 11. Record a command with environment variables
echo "\n--- 11. Recording a command with environment variables ---"
echo "\n>>> Running: ./$APP_NAME record -o $DEMO_DIR/env_test.vcr --with-env -- bash -c 'echo \"My custom var: $MY_CUSTOM_VAR\"'"
"$APP_NAME" record -o "$DEMO_DIR/env_test.vcr" --with-env -- bash -c 'echo "My custom var: $MY_CUSTOM_VAR"'
echo "<<< Command finished."
echo "\n>>> Running: ./$APP_NAME inspect $DEMO_DIR/env_test.vcr"
"$APP_NAME" inspect "$DEMO_DIR/env_test.vcr"
echo "<<< Command finished."

wait_for_keypress

echo "\n--- 11b. Replaying the command with environment variables ---"
echo "\n>>> Running: MY_CUSTOM_VAR='Hello from outside!' ./$APP_NAME replay $DEMO_DIR/env_test.vcr"
MY_CUSTOM_VAR='Hello from outside!' "$APP_NAME" replay "$DEMO_DIR/env_test.vcr"
echo "<<< Command finished."

wait_for_keypress

# 12. Automatic Caching and Fallback (The High-Value Feature)
echo "\n--- 12. Automatic Caching and Fallback ---"

# Clean up the specific voucher file first
echo "\n>>> Running: rm -f $DEMO_DIR/cache_test.vcr"
rm -f "$DEMO_DIR/cache_test.vcr"
echo "<<< Command finished."

wait_for_keypress

# 12a. First run: Cache miss, executes fallback, records new voucher
echo "\n--- 12a. First run: Cache Miss (Voucher missing) ---"
echo "\n>>> Running: ./$APP_NAME replay $DEMO_DIR/cache_test.vcr --fallback -- echo 'LIVE RUN: Cache was missing, now recorded.'"
"$APP_NAME" replay "$DEMO_DIR/cache_test.vcr" --fallback -- echo 'LIVE RUN: Cache was missing, now recorded.'
echo "<<< Command finished."
echo "\n>>> Running: ./$APP_NAME inspect $DEMO_DIR/cache_test.vcr"
"$APP_NAME" inspect "$DEMO_DIR/cache_test.vcr"
echo "<<< Command finished."

wait_for_keypress

# 12b. Second run: Cache hit, replays instantly
echo "\n--- 12b. Second run: Cache Hit (Replays instantly) ---"
echo "\n>>> Running: ./$APP_NAME replay $DEMO_DIR/cache_test.vcr --fallback -- echo 'LIVE RUN: Cache was missing, now recorded.'"
"$APP_NAME" replay "$DEMO_DIR/cache_test.vcr" --fallback -- echo 'LIVE RUN: Cache was missing, now recorded.'
echo "<<< Command finished."

wait_for_keypress

# 12c. Third run: Cache stale (TTL expired), executes fallback, refreshes cache
echo "\n--- 12c. Third run: Cache Stale (TTL expired) ---"
echo "\n>>> Running: ./$APP_NAME record -o $DEMO_DIR/stale_cache.vcr --ttl 1s -- echo 'STALE: This is the old cache.'"
"$APP_NAME" record -o "$DEMO_DIR/stale_cache.vcr" --ttl 1s -- echo 'STALE: This is the old cache.'
echo "<<< Command finished."
echo "\n>>> Running: sleep 2"
sleep 2
echo "<<< Command finished."
echo "\n>>> Running: ./$APP_NAME replay $DEMO_DIR/stale_cache.vcr --validate --fallback -- echo 'LIVE RUN: Cache was stale, now refreshed.'"
"$APP_NAME" replay "$DEMO_DIR/stale_cache.vcr" --validate --fallback -- echo 'LIVE RUN: Cache was stale, now refreshed.'
echo "<<< Command finished."
echo "\n>>> Running: ./$APP_NAME inspect $DEMO_DIR/stale_cache.vcr"
"$APP_NAME" inspect "$DEMO_DIR/stale_cache.vcr"
echo "<<< Command finished."

echo "======================================="
echo "  Demo Script Finished!"
echo "======================================="
