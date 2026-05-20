// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"go.etcd.io/bbolt"
)

// precheckEnvVar, when set on a process, causes the binary to attempt to open
// the bbolt database at the given path and exit before main() runs. The
// filestorage extension uses this to isolate bbolt panics (some of which occur
// in goroutines spawned inside bbolt.Open and therefore cannot be recovered
// from in-process) to a short-lived subprocess.
const precheckEnvVar = "OTELCOL_FILESTORAGE_PRECHECK_DB"

// precheckTimeoutEnvVar overrides the bbolt.Open timeout used inside the
// subprocess. The value must be a Go duration (e.g. "5s").
const precheckTimeoutEnvVar = "OTELCOL_FILESTORAGE_PRECHECK_TIMEOUT"

// Subprocess exit codes. The Go runtime exits with 2 on an unrecovered panic,
// so distinct codes are used for non-panic outcomes.
const (
	precheckExitOK        = 0 // bbolt.Open + Close succeeded.
	precheckExitPanic     = 2 // Set implicitly by the Go runtime on panic.
	precheckExitOpenError = 3 // bbolt.Open returned a non-panic error.
)

// errDBCorruption is returned by runPrecheck when the subprocess is observed
// to have crashed with the runtime panic exit code, which strongly suggests
// the bbolt database is corrupt in a way bbolt cannot open without panicking.
var errDBCorruption = errors.New("filestorage: bbolt database appears corrupt")

func init() {
	runPrecheckIfRequested()
}

// runPrecheckIfRequested is called from init(). When precheckEnvVar is set,
// the process attempts to open the bbolt database at the configured path and
// exits immediately. main() is never reached, so the rest of the collector
// never starts up in this short-lived child process.
//
// If bbolt.Open panics in a spawned goroutine, the Go runtime terminates the
// process with exit code 2; the parent process treats that exit code as a
// corruption signal.
func runPrecheckIfRequested() {
	path := os.Getenv(precheckEnvVar)
	if path == "" {
		return
	}

	timeout := 5 * time.Second
	if raw := os.Getenv(precheckTimeoutEnvVar); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			timeout = parsed
		}
	}

	opts := &bbolt.Options{
		Timeout:        timeout,
		NoFreelistSync: true,
		FreelistType:   bbolt.FreelistMapType,
	}

	db, err := bbolt.Open(path, 0o600, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "filestorage precheck: bbolt.Open returned error: %v\n", err)
		os.Exit(precheckExitOpenError)
	}
	if closeErr := db.Close(); closeErr != nil {
		fmt.Fprintf(os.Stderr, "filestorage precheck: bbolt.Close returned error: %v\n", closeErr)
		os.Exit(precheckExitOpenError)
	}
	os.Exit(precheckExitOK)
}

// runPrecheck attempts to open the bbolt database at path in a subprocess.
// It returns:
//
//   - nil if the database opened cleanly,
//   - errDBCorruption if the subprocess crashed with the runtime panic exit code
//     (strong signal of bbolt corruption that cannot be opened in-process), or
//   - a non-nil non-corruption error for any other failure (e.g. file lock,
//     permission denied, subprocess could not be started).
//
// The caller should only treat errDBCorruption as a reason to recreate the
// database. Other errors should typically be surfaced to the user.
func runPrecheck(ctx context.Context, path string, bboltTimeout time.Duration) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("determine executable path: %w", err)
	}

	// Allow the subprocess slightly more time than the bbolt timeout to start
	// up, parse options, and exit cleanly.
	subprocessTimeout := max(bboltTimeout+10*time.Second, 15*time.Second)

	cmdCtx, cancel := context.WithTimeout(ctx, subprocessTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, exe)
	cmd.Env = append(os.Environ(),
		precheckEnvVar+"="+path,
		precheckTimeoutEnvVar+"="+bboltTimeout.String(),
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	runErr := cmd.Run()
	if runErr == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		switch exitErr.ExitCode() {
		case precheckExitPanic:
			return errDBCorruption
		case precheckExitOpenError:
			return fmt.Errorf("precheck: subprocess could not open database: %w", runErr)
		default:
			return fmt.Errorf("precheck: subprocess exited with code %d: %w", exitErr.ExitCode(), runErr)
		}
	}
	return fmt.Errorf("precheck: subprocess failed to run: %w", runErr)
}
