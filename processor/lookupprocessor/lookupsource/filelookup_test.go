// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

// contentParser is a stand-in ParseFunc for these engine tests, in place of the
// real csv/yaml parsers. It stores the whole file under the key "val" (trimmed of
// surrounding whitespace), so Lookup("val") returns the current file contents.
func contentParser(content []byte) (map[string]any, error) {
	return map[string]any{"val": strings.TrimSpace(string(content))}, nil
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestFileLookupLoadAndLookup(t *testing.T) {
	fl := NewFileLookup(FileLookupSettings{Path: writeFile(t, "hello"), Parse: contentParser})
	require.NoError(t, fl.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, fl.Shutdown(t.Context())) }()

	val, found, err := fl.Lookup(t.Context(), "val")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "hello", val)

	_, found, err = fl.Lookup(t.Context(), "missing")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFileLookupNoReloadWhenIntervalZero(t *testing.T) {
	path := writeFile(t, "v1")
	fl := NewFileLookup(FileLookupSettings{Path: path, Parse: contentParser})
	require.NoError(t, fl.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, fl.Shutdown(t.Context())) }()

	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))

	// With reloading disabled the value must stay at the initial load.
	require.Never(t, func() bool {
		v, _, _ := fl.Lookup(t.Context(), "val")
		return v == "v2"
	}, 200*time.Millisecond, 20*time.Millisecond)
}

func TestFileLookupReloadAndCallback(t *testing.T) {
	path := writeFile(t, "v1")
	var successes, failures atomic.Int64
	fl := NewFileLookup(FileLookupSettings{
		Path:           path,
		ReloadInterval: 20 * time.Millisecond,
		Parse:          contentParser,
		OnReload: func(_ context.Context, success bool) {
			if success {
				successes.Add(1)
			} else {
				failures.Add(1)
			}
		},
	})
	require.NoError(t, fl.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, fl.Shutdown(t.Context())) }()

	require.NoError(t, os.WriteFile(path, []byte("v2"), 0o600))
	require.Eventually(t, func() bool {
		v, _, _ := fl.Lookup(t.Context(), "val")
		return v == "v2"
	}, 2*time.Second, 10*time.Millisecond)

	assert.Positive(t, successes.Load())
	assert.Zero(t, failures.Load())
}

func TestFileLookupKeepsLastGoodOnParseError(t *testing.T) {
	parse := func(content []byte) (map[string]any, error) {
		if strings.TrimSpace(string(content)) == "BAD" {
			return nil, errors.New("parse failed")
		}
		return contentParser(content)
	}
	path := writeFile(t, "good")
	var failures atomic.Int64
	fl := NewFileLookup(FileLookupSettings{
		Path:           path,
		ReloadInterval: 20 * time.Millisecond,
		Parse:          parse,
		// Logger left nil to exercise the no-op default on the reload error path.
		OnReload: func(_ context.Context, success bool) {
			if !success {
				failures.Add(1)
			}
		},
	})
	require.NoError(t, fl.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, fl.Shutdown(t.Context())) }()

	require.NoError(t, os.WriteFile(path, []byte("BAD"), 0o600))
	require.Eventually(t, func() bool { return failures.Load() > 0 }, 2*time.Second, 10*time.Millisecond)

	// The previous good data is retained after the failed reload.
	val, found, err := fl.Lookup(t.Context(), "val")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "good", val)
}

func TestFileLookupInitialLoadError(t *testing.T) {
	fl := NewFileLookup(FileLookupSettings{Path: "/nonexistent/path/data", Parse: contentParser})
	err := fl.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestFileLookupNilMapParse(t *testing.T) {
	parse := func([]byte) (map[string]any, error) { return nil, nil }
	fl := NewFileLookup(FileLookupSettings{Path: writeFile(t, "x"), Parse: parse})
	require.NoError(t, fl.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, fl.Shutdown(t.Context())) }()

	_, found, err := fl.Lookup(t.Context(), "anything")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFileLookupShutdownWithoutStart(t *testing.T) {
	fl := NewFileLookup(FileLookupSettings{Path: writeFile(t, "x"), Parse: contentParser})
	require.NoError(t, fl.Shutdown(t.Context()))
}
