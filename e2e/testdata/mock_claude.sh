#!/usr/bin/env bash
# mock_claude.sh — simulates the claude CLI binary for E2E testing.
#
# Environment variables:
#   MOCK_EXIT_CODE        — exit code (default: 0)
#   MOCK_DELAY_MS         — sleep before responding in milliseconds (default: 0)
#   MOCK_STDERR           — content written to stderr (default: empty)
#   MOCK_RESPONSE         — stdout response for executor calls (default: {"result":"mock ok"})
#   MOCK_PLANNER_RESPONSE — stdout response for planner calls (default: single-node DAG JSON)
#   MOCK_FAIL_COUNT       — fail first N calls then succeed (requires MOCK_COUNTER_FILE)
#   MOCK_COUNTER_FILE     — file path to persist call count across invocations
#
# Detection: if any argument contains "task planner" or "Decompose", it is
# treated as a planner call; otherwise it is an executor call.

set -euo pipefail

# --- Defaults ---
exit_code="${MOCK_EXIT_CODE:-0}"
delay_ms="${MOCK_DELAY_MS:-0}"
stderr_msg="${MOCK_STDERR:-}"
fail_count="${MOCK_FAIL_COUNT:-}"
counter_file="${MOCK_COUNTER_FILE:-}"

# Defaults containing braces must be assigned conditionally to avoid
# conflicts with the ${VAR:-default} closing-brace syntax.
if [ -n "${MOCK_RESPONSE+set}" ]; then
    response="$MOCK_RESPONSE"
else
    response='{"result":"mock ok"}'
fi

if [ -n "${MOCK_PLANNER_RESPONSE+set}" ]; then
    planner_response="$MOCK_PLANNER_RESPONSE"
else
    planner_response='[{"id":"task_1","task":"execute the task","depends":[]}]'
fi

# --- Delay ---
if [ "$delay_ms" -gt 0 ] 2>/dev/null; then
    # Convert milliseconds to seconds for sleep. Use awk for portable float math.
    sleep_secs=$(awk "BEGIN {printf \"%.3f\", $delay_ms / 1000}")
    sleep "$sleep_secs"
fi

# --- Retry support ---
# When both MOCK_FAIL_COUNT and MOCK_COUNTER_FILE are set, track invocations.
# Calls 1..MOCK_FAIL_COUNT fail with the configured exit code and stderr.
# Subsequent calls succeed (exit 0, normal response).
if [ -n "$fail_count" ] && [ -n "$counter_file" ]; then
    # Read current count (0 if file doesn't exist yet).
    if [ -f "$counter_file" ]; then
        current=$(cat "$counter_file")
    else
        current=0
    fi

    # Increment and persist.
    current=$((current + 1))
    echo "$current" > "$counter_file"

    if [ "$current" -le "$fail_count" ]; then
        # Fail this call.
        if [ -n "$stderr_msg" ]; then
            echo "$stderr_msg" >&2
        fi
        exit "${exit_code:-1}"
    else
        # Past the fail threshold — succeed regardless of MOCK_EXIT_CODE.
        exit_code=0
        stderr_msg=""
    fi
fi

# --- Detect planner vs executor ---
is_planner=false
for arg in "$@"; do
    if echo "$arg" | grep -q "task planner"; then
        is_planner=true
        break
    fi
    if echo "$arg" | grep -q "Decompose"; then
        is_planner=true
        break
    fi
done

# --- Stderr ---
if [ -n "$stderr_msg" ]; then
    echo "$stderr_msg" >&2
fi

# --- Stdout ---
if [ "$is_planner" = true ]; then
    echo "$planner_response"
else
    echo "$response"
fi

exit "$exit_code"
