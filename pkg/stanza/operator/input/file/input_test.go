// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

// AddFileResolvedFields tests that the `log.file.name_resolved` and `log.file.path_resolved` fields are included
// when IncludeFileNameResolved and IncludeFilePathResolved are set to true
func TestAddFileResolvedFields(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlinks usage disabled for now. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21088")
	}
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
		cfg.IncludeFileName = true
		cfg.IncludeFilePath = true
		cfg.IncludeFileNameResolved = true
		cfg.IncludeFilePathResolved = true
		cfg.IncludeFileOwnerName = runtime.GOOS != "windows"
		cfg.IncludeFileOwnerGroupName = runtime.GOOS != "windows"
	})

	// Create temp dir with log file
	dir := t.TempDir()

	file, err := os.CreateTemp(dir, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
	})

	// Create symbolic link in monitored directory
	symLinkPath := filepath.Join(tempDir, "symlink")
	err = os.Symlink(file.Name(), symLinkPath)
	require.NoError(t, err)

	// Populate data
	writeString(t, file, "testlog\n")

	// Resolve path
	realPath, err := filepath.EvalSymlinks(file.Name())
	require.NoError(t, err)
	resolved, err := filepath.Abs(realPath)
	require.NoError(t, err)

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	e := waitForOne(t, logReceived)
	require.Equal(t, filepath.Base(symLinkPath), e.Attributes["log.file.name"])
	require.Equal(t, symLinkPath, e.Attributes["log.file.path"])
	require.Equal(t, filepath.Base(resolved), e.Attributes["log.file.name_resolved"])
	require.Equal(t, resolved, e.Attributes["log.file.path_resolved"])
	if runtime.GOOS != "windows" {
		require.NotNil(t, e.Attributes["log.file.owner.name"])
		require.NotNil(t, e.Attributes["log.file.owner.group.name"])
	}
}

// AddFileRecordNumber tests that the `log.file.record_number` is correctly included
// when IncludeFileRecordNumber is set to true
func TestAddFileRecordNumber(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
		cfg.IncludeFileRecordNumber = true
	})

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\ntestlog3\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	e := waitForOne(t, logReceived)
	require.Equal(t, "testlog1", e.Body)
	require.Equal(t, int64(1), e.Attributes["log.file.record_number"])

	e = waitForOne(t, logReceived)
	require.Equal(t, "testlog2", e.Body)
	require.Equal(t, int64(2), e.Attributes["log.file.record_number"])

	e = waitForOne(t, logReceived)
	require.Equal(t, "testlog3", e.Body)
	require.Equal(t, int64(3), e.Attributes["log.file.record_number"])

	// Write 3 more entries
	writeString(t, temp, "testlog4\ntestlog5\ntestlog6\n")

	e = waitForOne(t, logReceived)
	require.Equal(t, "testlog4", e.Body)
	require.Equal(t, int64(4), e.Attributes["log.file.record_number"])

	e = waitForOne(t, logReceived)
	require.Equal(t, "testlog5", e.Body)
	require.Equal(t, int64(5), e.Attributes["log.file.record_number"])

	e = waitForOne(t, logReceived)
	require.Equal(t, "testlog6", e.Body)
	require.Equal(t, int64(6), e.Attributes["log.file.record_number"])
}

// ReadExistingLogs tests that, when starting from beginning, we
// read all the lines that are already there
func TestReadExistingLogs(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// TestReadUsingNopEncoding tests when nop encoding is set, that the splitfunction returns all bytes unchanged.
func TestReadUsingNopEncoding(t *testing.T) {
	tcs := []struct {
		testName string
		input    []byte
		test     func(*testing.T, chan *entry.Entry)
	}{
		{
			"simple",
			[]byte("testlog1"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog1"))
			},
		},
		{
			"longer than maxlogsize",
			[]byte("testlog1testlog2testlog3"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog1"))
				waitForByteMessage(t, c, []byte("testlog2"))
				waitForByteMessage(t, c, []byte("testlog3"))
			},
		},
		{
			"doesn't hit max log size before eof",
			[]byte("testlog1testlog2test"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog1"))
				waitForByteMessage(t, c, []byte("testlog2"))
				waitForByteMessage(t, c, []byte("test"))
			},
		},
		{
			"special characters",
			[]byte("testlog1\n\ttestlog2\n\t"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog1"))
				waitForByteMessage(t, c, []byte("\n\ttestlo"))
				waitForByteMessage(t, c, []byte("g2\n\t"))
			},
		},
	}

	t.Parallel()

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
				cfg.MaxLogSize = 8
				cfg.Encoding = "nop"
			})
			// Create a file, then start
			temp := openTemp(t, tempDir)
			bytesWritten, err := temp.Write(tc.input)
			require.Positive(t, bytesWritten)
			require.NoError(t, err)
			require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			tc.test(t, logReceived)
		})
	}
}

// ReadNewLogs tests that, after starting, if a new file is created
// all the entries in that file are read from the beginning
func TestReadNewLogs(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// Create a new file
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog\n")

	// Expect the message to come through
	waitForMessage(t, logReceived, "testlog")
}

// ReadExistingAndNewLogs tests that, on startup, if start_at
// is set to `beginning`, we read the logs that are there, and
// we read any additional logs that are written after startup
func TestReadExistingAndNewLogs(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	// Start with a file with an entry in it, and expect that entry
	// to come through when we poll for the first time
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForMessage(t, logReceived, "testlog1")

	// Write a second entry, and expect that entry to come through
	// as well
	writeString(t, temp, "testlog2\n")
	waitForMessage(t, logReceived, "testlog2")
}

// StartAtEnd tests that when `start_at` is configured to `end`,
// we don't read any entries that were in the file before startup
func TestStartAtEnd(t *testing.T) {
	t.Parallel()

	var pollInterval time.Duration
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
		cfg.StartAt = "end"
		pollInterval = cfg.PollInterval
	})

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	time.Sleep(2 * pollInterval)

	expectNoMessages(t, logReceived)

	// Expect any new entries after the first poll
	writeString(t, temp, "testlog2\n")
	waitForMessage(t, logReceived, "testlog2")
}

// SkipEmpty tests that the any empty lines are skipped
func TestSkipEmpty(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}
