#!/bin/sh
# entrypoint.sh — Test runner entrypoint for the otb security/fuzz container.
#
# Modes (controlled by TEST_MODE env var):
#   unit      — run all unit tests with race detector (default)
#   fuzz      — run all fuzz targets for FUZZ_SECONDS each
#   security  — run black-box security tests against the compiled binary
#   all       — unit + security (fuzz must be explicit; it's time-bounded)

set -e

FUZZ_SECONDS="${FUZZ_SECONDS:-30}"
TEST_TIMEOUT="${TEST_TIMEOUT:-120}"
TEST_MODE="${TEST_MODE:-unit}"

BIN=/app/bin/otb
RESULTS_DIR=/results
mkdir -p "$RESULTS_DIR"

log() { printf '[otb-test] %s\n' "$*"; }

run_unit() {
    log "Running unit tests (race detector, timeout=${TEST_TIMEOUT}s)..."
    cd /app
    go test -race -count=1 -timeout="${TEST_TIMEOUT}s" \
        ./internal/parser/... \
        ./internal/reader/... \
        ./internal/sanitize/... \
        ./internal/vault/... \
        ./internal/writer/... \
        -v 2>&1 | tee "$RESULTS_DIR/unit.log"
    log "Unit tests complete."
}

run_fuzz() {
    log "Running fuzz targets (${FUZZ_SECONDS}s each)..."
    cd /app

    fuzz_targets() {
        # Format: "package FuzzFunctionName"
        echo "internal/parser FuzzParseTaskLine"
        echo "internal/parser FuzzParseProjectFile"
        echo "internal/sanitize FuzzForDisplay"
        echo "internal/writer FuzzChangeTaskStatus"
        echo "internal/writer FuzzAddTaskComment"
        echo "internal/writer FuzzChangeTaskStatus_SourceFilePath"
        echo "internal/vault FuzzVaultResolve"
        echo "internal/vault FuzzIsUnderVault"
    }

    FUZZ_FAILED=0
    while IFS=' ' read -r pkg func; do
        log "  fuzzing ./$pkg -run='^$' -fuzz='^${func}$' for ${FUZZ_SECONDS}s..."
        if go test "./$pkg" \
            -run='^$' \
            -fuzz="^${func}$" \
            -fuzztime="${FUZZ_SECONDS}s" \
            -timeout="$((FUZZ_SECONDS + 30))s" \
            2>&1 | tee "$RESULTS_DIR/fuzz-${func}.log"; then
            log "  [OK] $func"
        else
            log "  [FAIL] $func — check $RESULTS_DIR/fuzz-${func}.log"
            FUZZ_FAILED=$((FUZZ_FAILED + 1))
        fi
    done <<EOF
$(fuzz_targets)
EOF

    if [ "$FUZZ_FAILED" -gt 0 ]; then
        log "Fuzz testing: $FUZZ_FAILED target(s) FAILED."
        return 1
    fi
    log "Fuzz testing: all targets passed."
}

run_security() {
    log "Running security black-box tests..."

    if [ ! -f "$BIN" ]; then
        log "ERROR: binary not found at $BIN — building..."
        cd /app
        CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN" .
    fi

    cd /app
    go test -v -count=1 -timeout="${TEST_TIMEOUT}s" \
        ./internal/security/... \
        2>&1 | tee "$RESULTS_DIR/security.log"
    log "Security tests complete."
}

case "$TEST_MODE" in
    unit)
        run_unit
        ;;
    fuzz)
        run_fuzz
        ;;
    security)
        run_security
        ;;
    all)
        run_unit
        run_security
        ;;
    fuzz+security)
        run_fuzz
        run_security
        ;;
    *)
        log "Unknown TEST_MODE='$TEST_MODE'. Valid: unit | fuzz | security | all | fuzz+security"
        exit 1
        ;;
esac

log "Done. Results written to $RESULTS_DIR/"
