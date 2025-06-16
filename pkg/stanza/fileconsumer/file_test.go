// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

// TestDefaultBehaviors
// - Files are read starting from the end.
// - Logs are tokenized based on newlines.
// - Leading and trailing whitespace is trimmed.
// - log.file.name is included as an attribute.
// - Incomplete logs are flushed after a default flush period.
func TestDefaultBehaviors(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	operator, sink := testManager(t, cfg)

	temp := filetest.OpenTemp(t, tempDir)
	tempName := filepath.Base(temp.Name())
	filetest.WriteString(t, temp, " testlog1 \n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// Should not emit the pre-existing token, even after flush period
	sink.ExpectNoCallsUntil(t, reader.DefaultFlushPeriod)

	// Complete token should be emitted quickly
	filetest.WriteString(t, temp, " testlog2 \n")
	token, attributes := sink.NextCall(t)
	assert.Equal(t, []byte("testlog2"), token)
	assert.Len(t, attributes, 1)
	assert.Equal(t, tempName, attributes[attrs.LogFileName])

	// Incomplete token should not be emitted until after flush period
	filetest.WriteString(t, temp, " testlog3 ")
	sink.ExpectNoCallsUntil(t, reader.DefaultFlushPeriod/2)
	time.Sleep(reader.DefaultFlushPeriod)

	token, attributes = sink.NextCall(t)
	assert.Equal(t, []byte("testlog3"), token)
	assert.Len(t, attributes, 1)
	assert.Equal(t, tempName, attributes[attrs.LogFileName])
}

// ReadExistingLogs tests that, when starting from beginning, we
// read all the lines that are already there
func TestReadExistingLogs(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	// Create a file, then start
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog1\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte("testlog1"))
	sink.ExpectToken(t, []byte("testlog2"))
}

// TestReadUsingNopEncoding tests when nop encoding is set, that the splitfunction returns all bytes unchanged.
func TestReadUsingNopEncoding(t *testing.T) {
	tcs := []struct {
		testName string
		input    []byte
		test     func(*testing.T, *emittest.Sink)
	}{
		{
			"simple",
			[]byte("testlog1"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog1"))
			},
		},
		{
			"longer than maxlogsize",
			[]byte("testlog1testlog2testlog3"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog1"))
				sink.ExpectToken(t, []byte("testlog2"))
				sink.ExpectToken(t, []byte("testlog3"))
			},
		},
		{
			"doesn't hit max log size before eof",
			[]byte("testlog1testlog2test"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog1"))
				sink.ExpectToken(t, []byte("testlog2"))
				sink.ExpectToken(t, []byte("test"))
			},
		},
		{
			"special characters",
			[]byte("testlog1\n\ttestlog2\n\t"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog1"))
				sink.ExpectToken(t, []byte("\n\ttestlo"))
				sink.ExpectToken(t, []byte("g2\n\t"))
			},
		},
	}

	t.Parallel()

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			tempDir := t.TempDir()
			cfg := NewConfig().includeDir(tempDir)
			cfg.StartAt = "beginning"
			cfg.MaxLogSize = 8
			cfg.Encoding = "nop"
			operator, sink := testManager(t, cfg)

			// Create a file, then start
			temp := filetest.OpenTemp(t, tempDir)
			bytesWritten, err := temp.Write(tc.input)
			require.Positive(t, bytesWritten)
			require.NoError(t, err)
			require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			tc.test(t, sink)
		})
	}
}

func TestNopEncodingDifferentLogSizes(t *testing.T) {
	tcs := []struct {
		testName   string
		input      []byte
		test       func(*testing.T, *emittest.Sink)
		maxLogSize helper.ByteSize
	}{
		{
			"same size",
			[]byte("testlog1"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog1"))
			},
			8,
		},
		{
			"massive log size",
			[]byte("testlog1"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog1"))
			},
			8000000,
		},
		{
			"slightly larger log size",
			[]byte("testlog1"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog1"))
			},
			9,
		},
		{
			"slightly smaller log size",
			[]byte("testlog1"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("testlog"))
				sink.ExpectToken(t, []byte("1"))
			},
			7,
		},
		{
			"tiny log size",
			[]byte("testlog1"),
			func(t *testing.T, sink *emittest.Sink) {
				sink.ExpectToken(t, []byte("t"))
				sink.ExpectToken(t, []byte("e"))
				sink.ExpectToken(t, []byte("s"))
				sink.ExpectToken(t, []byte("t"))
				sink.ExpectToken(t, []byte("l"))
				sink.ExpectToken(t, []byte("o"))
				sink.ExpectToken(t, []byte("g"))
				sink.ExpectToken(t, []byte("1"))
			},
			1,
		},
	}

	t.Parallel()

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			tempDir := t.TempDir()
			cfg := NewConfig().includeDir(tempDir)
			cfg.StartAt = "beginning"
			cfg.MaxLogSize = tc.maxLogSize
			cfg.Encoding = "nop"
			operator, sink := testManager(t, cfg)

			// Create a file, then start
			temp := filetest.OpenTemp(t, tempDir)
			bytesWritten, err := temp.Write(tc.input)
			require.Positive(t, bytesWritten)
			require.NoError(t, err)
			require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			tc.test(t, sink)
		})
	}
}

// ReadNewLogs tests that, after starting, if a new file is created
// all the entries in that file are read from the beginning
func TestReadNewLogs(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	// Poll once so we know this isn't a new file
	operator.poll(context.Background())

	// Create a new file
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog\n")

	// Poll a second time after the file has been created
	operator.poll(context.Background())

	// Expect the message to come through
	sink.ExpectToken(t, []byte("testlog"))
}

// ReadExistingAndNewLogs tests that, on startup, if start_at
// is set to `beginning`, we read the logs that are there, and
// we read any additional logs that are written after startup
func TestReadExistingAndNewLogs(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	// Start with a file with an entry in it, and expect that entry
	// to come through when we poll for the first time
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog1\n")
	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))

	// Write a second entry, and expect that entry to come through
	// as well
	filetest.WriteString(t, temp, "testlog2\n")
	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog2"))
}

// StartAtEnd tests that when `start_at` is configured to `end`,
// we don't read any entries that were in the file before startup
func TestStartAtEnd(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog1\n")

	// Expect no entries on the first poll
	operator.poll(context.Background())
	sink.ExpectNoCalls(t)

	// Expect any new entries after the first poll
	filetest.WriteString(t, temp, "testlog2\n")
	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog2"))
}

// TestSymlinkedFiles tests reading from a single file that's actually a symlink
// to another file, while the symlink target is changed frequently, reads all
// the logs from all the files ever targeted by that symlink.
func TestSymlinkedFiles(t *testing.T) {
	if runtime.GOARCH == "arm64" {
		t.Skip("Failing consistently on ARM64. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34494")
	}

	if runtime.GOOS == "windows" {
		t.Skip("Time sensitive tests disabled for now on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32715#issuecomment-2107737828")
	}

	t.Parallel()

	// Create 30 files with a predictable naming scheme, each containing
	// 100 log lines.
	const numFiles = 30
	const logLinesPerFile = 100
	const pollInterval = 10 * time.Millisecond
	tempDir := t.TempDir()
	expectedTokens := [][]byte{}
	for i := 1; i <= numFiles; i++ {
		expectedTokensBatch := symlinkTestCreateLogFile(t, tempDir, i, logLinesPerFile)
		expectedTokens = append(expectedTokens, expectedTokensBatch...)
	}

	targetTempDir := t.TempDir()
	symlinkFilePath := filepath.Join(targetTempDir, "sym.log")
	cfg := NewConfig().includeDir(targetTempDir)
	cfg.StartAt = "beginning"
	cfg.PollInterval = pollInterval
	sink := emittest.NewSink(emittest.WithCallBuffer(numFiles * logLinesPerFile))
	operator := testManagerWithSink(t, cfg, sink)

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectNoCalls(t)

	// Create and update symlink to each of the files over time.
	for i := 1; i <= numFiles; i++ {
		targetLogFilePath := filepath.Join(tempDir, fmt.Sprintf("%d.log", i))
		require.NoError(t, os.Symlink(targetLogFilePath, symlinkFilePath))
		// The sleep time here must be larger than the poll_interval value
		time.Sleep(pollInterval + 1*time.Millisecond)
		require.NoError(t, os.Remove(symlinkFilePath))
	}
	sink.ExpectTokens(t, expectedTokens...)
}

// StartAtEndNewFile tests that when `start_at` is configured to `end`,
// a file created after the operator has been started is read from the
// beginning
func TestStartAtEndNewFile(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	operator.poll(context.Background())
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))
	sink.ExpectToken(t, []byte("testlog2"))
}

// NoNewline tests that an entry will still be sent eventually
// even if the file doesn't end in a newline
func TestNoNewline(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.FlushPeriod = time.Nanosecond
	operator, sink := testManager(t, cfg)

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog1\ntestlog2")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte("testlog1"))
	sink.ExpectToken(t, []byte("testlog2"))
}

// TestEmptyLine tests that the any empty lines are consumed
func TestEmptyLine(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog1\n\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte("testlog1"))
	sink.ExpectToken(t, []byte(""))
	sink.ExpectToken(t, []byte("testlog2"))
}

// TestMultipleEmpty tests that multiple empty lines
// can be consumed without the operator becoming stuck
func TestMultipleEmpty(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "\n\ntestlog1\n\n\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte(""))
	sink.ExpectToken(t, []byte(""))
	sink.ExpectToken(t, []byte("testlog1"))
	sink.ExpectToken(t, []byte(""))
	sink.ExpectToken(t, []byte(""))
	sink.ExpectToken(t, []byte("testlog2"))
	sink.ExpectNoCallsUntil(t, time.Second)
}

// TestLeadingEmpty tests that the the operator handles a leading
// newline, and does not read the file multiple times
func TestLeadingEmpty(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "\ntestlog1\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte(""))
	sink.ExpectToken(t, []byte("testlog1"))
	sink.ExpectToken(t, []byte("testlog2"))
	sink.ExpectNoCallsUntil(t, time.Second)
}

// SplitWrite tests a line written in two writes
// close together still is read as a single entry
func TestSplitWrite(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog1")

	operator.poll(context.Background())

	filetest.WriteString(t, temp, "testlog2\n")

	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog1testlog2"))
}

func TestIgnoreEmptyFiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temp := filetest.OpenTemp(t, tempDir)
	temp2 := filetest.OpenTemp(t, tempDir)
	temp3 := filetest.OpenTemp(t, tempDir)
	temp4 := filetest.OpenTemp(t, tempDir)

	filetest.WriteString(t, temp, "testlog1\n")
	filetest.WriteString(t, temp3, "testlog2\n")
	operator.poll(context.Background())

	sink.ExpectTokens(t, []byte("testlog1"), []byte("testlog2"))

	filetest.WriteString(t, temp2, "testlog3\n")
	filetest.WriteString(t, temp4, "testlog4\n")
	operator.poll(context.Background())

	sink.ExpectTokens(t, []byte("testlog3"), []byte("testlog4"))
}

func TestDecodeBufferIsResized(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	temp := filetest.OpenTemp(t, tempDir)
	expected := filetest.TokenWithLength(1<<12 + 1)
	filetest.WriteString(t, temp, string(expected)+"\n")

	sink.ExpectToken(t, expected)
}

func TestMultiFileSimple(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	temp1 := filetest.OpenTemp(t, tempDir)
	temp2 := filetest.OpenTemp(t, tempDir)

	filetest.WriteString(t, temp1, "testlog1\n")
	filetest.WriteString(t, temp2, "testlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectTokens(t, []byte("testlog1"), []byte("testlog2"))
}

func TestMultiFileSort(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.OrderingCriteria = matcher.OrderingCriteria{
		Regex: `.*(?P<value>\d)`,
		SortBy: []matcher.Sort{
			{
				SortType: "numeric",
				RegexKey: "value",
			},
		},
	}

	operator, sink := testManager(t, cfg)

	temp1 := filetest.OpenTempWithPattern(t, tempDir, ".*log1")
	temp2 := filetest.OpenTempWithPattern(t, tempDir, ".*log2")

	filetest.WriteString(t, temp1, "testlog1\n")
	filetest.WriteString(t, temp2, "testlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectTokens(t, []byte("testlog2"))
	sink.ExpectNoCalls(t)
}

func TestMultiFileSortTimestamp(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.OrderingCriteria = matcher.OrderingCriteria{
		Regex: `.(?P<value>\d{10})\.log`,
		SortBy: []matcher.Sort{
			{
				SortType: "timestamp",
				RegexKey: `value`,
				Layout:   "%Y%m%d%H",
			},
		},
	}

	operator, sink := testManager(t, cfg)

	temp1 := filetest.OpenTempWithPattern(t, tempDir, ".*2023020602.log")
	temp2 := filetest.OpenTempWithPattern(t, tempDir, ".*2023020603.log")

	filetest.WriteString(t, temp1, "testlog1\n")
	filetest.WriteString(t, temp2, "testlog2\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectTokens(t, []byte("testlog2"))
	sink.ExpectNoCalls(t)
}

func TestMultiFileParallel_PreloadedFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	numFiles := 10
	numMessages := 100

	expected := make([][]byte, 0, numFiles*numMessages)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			expected = append(expected, []byte(getMessage(i, j)))
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < numFiles; i++ {
		temp := filetest.OpenTemp(t, tempDir)
		wg.Add(1)
		go func(tf *os.File, f int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				filetest.WriteString(t, tf, getMessage(f, j)+"\n")
			}
		}(temp, i)
	}

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectTokens(t, expected...)
	wg.Wait()
}

func TestMultiFileParallel_LiveFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	numFiles := 10
	numMessages := 100

	expected := make([][]byte, 0, numFiles*numMessages)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			expected = append(expected, []byte(getMessage(i, j)))
		}
	}

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	temps := make([]*os.File, 0, numFiles)
	for i := 0; i < numFiles; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}

	var wg sync.WaitGroup
	for i, temp := range temps {
		wg.Add(1)
		go func(tf *os.File, f int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				filetest.WriteString(t, tf, getMessage(f, j)+"\n")
			}
		}(temp, i)
	}

	sink.ExpectTokens(t, expected...)
	wg.Wait()
}

func TestRestartOffsets(t *testing.T) {
	testCases := []struct {
		name       string
		startAt    string
		lineLength int
	}{
		{"start_at_beginning_short", "beginning", 20},
		{"start_at_end_short", "end", 20},
		{"start_at_beginning_long", "beginning", 2000},
		{"start_at_end_short", "end", 2000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			cfg := NewConfig().includeDir(tempDir)
			cfg.StartAt = tc.startAt

			persister := testutil.NewUnscopedMockPersister()

			logFile := filetest.OpenTemp(t, tempDir)

			before1stRun := filetest.TokenWithLength(tc.lineLength)
			during1stRun := filetest.TokenWithLength(tc.lineLength)
			duringRestart := filetest.TokenWithLength(tc.lineLength)
			during2ndRun := filetest.TokenWithLength(tc.lineLength)

			operatorOne, sink1 := testManager(t, cfg)
			filetest.WriteString(t, logFile, string(before1stRun)+"\n")
			require.NoError(t, operatorOne.Start(persister))
			if tc.startAt == "beginning" {
				sink1.ExpectToken(t, before1stRun)
			} else {
				sink1.ExpectNoCallsUntil(t, 500*time.Millisecond)
			}
			filetest.WriteString(t, logFile, string(during1stRun)+"\n")
			sink1.ExpectToken(t, during1stRun)
			require.NoError(t, operatorOne.Stop())

			filetest.WriteString(t, logFile, string(duringRestart)+"\n")

			operatorTwo, sink2 := testManager(t, cfg)
			require.NoError(t, operatorTwo.Start(persister))
			sink2.ExpectToken(t, duringRestart)
			filetest.WriteString(t, logFile, string(during2ndRun)+"\n")
			sink2.ExpectToken(t, during2ndRun)
			require.NoError(t, operatorTwo.Stop())
		})
	}
}

func TestManyLogsDelivered(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	count := 1000
	expectedTokens := make([]string, 0, count)
	for i := 0; i < count; i++ {
		expectedTokens = append(expectedTokens, strconv.Itoa(i))
	}

	// Start the operator
	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// Write lots of logs
	temp := filetest.OpenTemp(t, tempDir)
	for _, message := range expectedTokens {
		_, err := temp.WriteString(message + "\n")
		require.NoError(t, err)
	}

	// Expect each of them to come through once
	for _, message := range expectedTokens {
		sink.ExpectToken(t, []byte(message))
	}
	sink.ExpectNoCalls(t)
}

func TestFileBatching(t *testing.T) {
	t.Parallel()

	files := 100
	linesPerFile := 10
	maxConcurrentFiles := 20
	// Explicitly setting maxBatches to ensure a value of 0 does not enforce a limit
	maxBatches := 0

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.MaxConcurrentFiles = maxConcurrentFiles
	cfg.MaxBatches = maxBatches
	sink := emittest.NewSink(emittest.WithCallBuffer(files * linesPerFile))
	operator := testManagerWithSink(t, cfg, sink)
	operator.persister = testutil.NewUnscopedMockPersister()

	temps := make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}

	// Write logs to each file
	expectedTokens := make([][]byte, 0, files*linesPerFile)
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", filetest.TokenWithLength(100), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
			expectedTokens = append(expectedTokens, []byte(message))
		}
	}

	// Poll and wait for all lines
	operator.poll(context.Background())
	actualTokens := make([][]byte, 0, files*linesPerFile)
	actualTokens = append(actualTokens, sink.NextTokens(t, len(expectedTokens))...)
	require.ElementsMatch(t, expectedTokens, actualTokens)

	// Write more logs to each file so we can validate that all files are still known
	expectedTokens = make([][]byte, 0, files*linesPerFile)
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", filetest.TokenWithLength(20), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
			expectedTokens = append(expectedTokens, []byte(message))
		}
	}

	// Poll again and wait for all new lines
	operator.poll(context.Background())
	actualTokens = make([][]byte, 0, files*linesPerFile)
	actualTokens = append(actualTokens, sink.NextTokens(t, len(expectedTokens))...)
	require.ElementsMatch(t, expectedTokens, actualTokens)
}

func TestMaxConcurrentFilesOne(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	temp1 := filetest.OpenTemp(t, tempDir)
	_, err := temp1.WriteString("file 0: written before start\n")
	require.NoError(t, err)

	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.MaxConcurrentFiles = 1

	sink := emittest.NewSink()
	operator := testManagerWithSink(t, cfg, sink)
	operator.persister = testutil.NewUnscopedMockPersister()
	operator.poll(context.Background())
	sink.ExpectTokens(t, []byte("file 0: written before start"))
}

func TestFileBatchingRespectsStartAtEnd(t *testing.T) {
	t.Parallel()

	initFiles := 10
	moreFiles := 10
	maxConcurrentFiles := 2

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "end"
	cfg.MaxConcurrentFiles = maxConcurrentFiles

	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temps := make([]*os.File, 0, initFiles+moreFiles)
	for i := 0; i < initFiles; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}

	// Write one log to each file
	for i, temp := range temps {
		message := fmt.Sprintf("file %d: %s", i, "written before start")
		_, err := temp.WriteString(message + "\n")
		require.NoError(t, err)
	}

	// Poll and expect no logs
	operator.poll(context.Background())
	sink.ExpectNoCalls(t)

	// Create some more files
	for i := 0; i < moreFiles; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}

	// Write a log to each file
	expectedTokens := make([][]byte, 0, initFiles+moreFiles)
	for i, temp := range temps {
		message := fmt.Sprintf("file %d: %s", i, "written after start")
		_, err := temp.WriteString(message + "\n")
		require.NoError(t, err)
		expectedTokens = append(expectedTokens, []byte(message))
	}

	// Poll again and expect one line from each file.
	operator.poll(context.Background())
	sink.ExpectTokens(t, expectedTokens...)
}

func TestEncodings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		contents []byte
		encoding string
		expected [][]byte
	}{
		{
			"Nop",
			[]byte{0xc5, '\n'},
			"nop",
			[][]byte{{0xc5, '\n'}},
		},
		{
			"InvalidUTFReplacement",
			[]byte{0xc5, '\n'},
			"utf8",
			[][]byte{{0xef, 0xbf, 0xbd}},
		},
		{
			"InvalidUTFWithoutReplacement",
			[]byte{0xc5, '\n'},
			"utf8-raw",
			[][]byte{{0xc5}},
		},
		{
			"ValidUTF8",
			[]byte("foo\n"),
			"utf8",
			[][]byte{[]byte("foo")},
		},
		{
			"ChineseCharacter",
			[]byte{230, 138, 152, '\n'}, // 折\n
			"utf8",
			[][]byte{{230, 138, 152}},
		},
		{
			"SmileyFaceUTF16",
			[]byte{216, 61, 222, 0, 0, 10}, // 😀\n
			"utf-16be",
			[][]byte{{240, 159, 152, 128}},
		},
		{
			"SmileyFaceNewlineUTF16",
			[]byte{216, 61, 222, 0, 0, 10, 0, 102, 0, 111, 0, 111}, // 😀\nfoo
			"utf-16be",
			[][]byte{{240, 159, 152, 128}, {102, 111, 111}},
		},
		{
			"SmileyFaceNewlineUTF16LE",
			[]byte{61, 216, 0, 222, 10, 0, 102, 0, 111, 0, 111, 0}, // 😀\nfoo
			"utf-16le",
			[][]byte{{240, 159, 152, 128}, {102, 111, 111}},
		},
		{
			"ChineseCharacterBig5",
			[]byte{167, 233, 10}, // 折\n
			"big5",
			[][]byte{{230, 138, 152}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			cfg := NewConfig().includeDir(tempDir)
			cfg.StartAt = "beginning"
			cfg.Encoding = tc.encoding
			operator, sink := testManager(t, cfg)

			// Populate the file
			temp := filetest.OpenTemp(t, tempDir)
			_, err := temp.Write(tc.contents)
			require.NoError(t, err)

			require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			sink.ExpectTokens(t, tc.expected...)
		})
	}
}

func TestDeleteAfterRead(t *testing.T) {
	t.Parallel()

	files := 10
	linesPerFile := 10
	totalLines := files * linesPerFile

	tempDir := t.TempDir()
	temps := make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}

	expectedTokens := make([][]byte, 0, totalLines)
	actualTokens := make([][]byte, 0, totalLines)
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			line := filetest.TokenWithLength(100)
			message := fmt.Sprintf("%s %d %d", line, i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
			expectedTokens = append(expectedTokens, []byte(message))
		}
		require.NoError(t, temp.Close())
	}

	require.NoError(t, featuregate.GlobalRegistry().Set(allowFileDeletion.ID(), true))

	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.DeleteAfterRead = true
	sink := emittest.NewSink(emittest.WithCallBuffer(totalLines))
	operator := testManagerWithSink(t, cfg, sink)
	operator.persister = testutil.NewUnscopedMockPersister()
	operator.poll(context.Background())
	actualTokens = append(actualTokens, sink.NextTokens(t, totalLines)...)

	require.ElementsMatch(t, expectedTokens, actualTokens)

	for _, temp := range temps {
		_, err := os.Stat(temp.Name())
		require.True(t, os.IsNotExist(err))
	}

	// Make more files to ensure deleted files do not cause problems on next poll
	temps = make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}

	expectedTokens = make([][]byte, 0, totalLines)
	actualTokens = make([][]byte, 0, totalLines)
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			line := filetest.TokenWithLength(200)
			message := fmt.Sprintf("%s %d %d", line, i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
			expectedTokens = append(expectedTokens, []byte(message))
		}
		require.NoError(t, temp.Close())
	}

	operator.poll(context.Background())
	actualTokens = append(actualTokens, sink.NextTokens(t, totalLines)...)

	require.ElementsMatch(t, expectedTokens, actualTokens)

	for _, temp := range temps {
		_, err := os.Stat(temp.Name())
		require.True(t, os.IsNotExist(err))
	}
}

func TestMaxBatching(t *testing.T) {
	t.Parallel()

	files := 50
	linesPerFile := 10
	maxConcurrentFiles := 20
	maxBatchFiles := maxConcurrentFiles / 2
	maxBatches := 2

	expectedMaxFilesPerPoll := maxBatches * maxBatchFiles

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.MaxConcurrentFiles = maxConcurrentFiles
	cfg.MaxBatches = maxBatches
	sink := emittest.NewSink(emittest.WithCallBuffer(files * linesPerFile))
	operator := testManagerWithSink(t, cfg, sink)
	operator.persister = testutil.NewUnscopedMockPersister()

	temps := make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, filetest.OpenTemp(t, tempDir))
	}

	// Write logs to each file
	numExpectedTokens := expectedMaxFilesPerPoll * linesPerFile
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", filetest.TokenWithLength(100), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
		}
	}

	// Poll and wait for all lines
	operator.poll(context.Background())
	actualTokens := make([][]byte, 0, numExpectedTokens)
	actualTokens = append(actualTokens, sink.NextTokens(t, numExpectedTokens)...)
	require.Len(t, actualTokens, numExpectedTokens)

	// Write more logs to each file so we can validate that all files are still known
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", filetest.TokenWithLength(20), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
		}
	}

	// Poll again and wait for all new lines
	operator.poll(context.Background())
	actualTokens = make([][]byte, 0, numExpectedTokens)
	actualTokens = append(actualTokens, sink.NextTokens(t, numExpectedTokens)...)
	require.Len(t, actualTokens, numExpectedTokens)
}

// TestReadExistingLogsWithHeader tests that, when starting from beginning, we
// read all the lines that are already there, and parses the headers
func TestReadExistingLogsWithHeader(t *testing.T) {
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg = cfg.withHeader("^#", "(?P<header_key>[A-z]+): (?P<header_value>[A-z]+)")

	operator, sink := testManager(t, cfg)

	// Create a file, then start
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "#headerField: headerValue\ntestlog\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectCall(t, []byte("testlog"), map[string]any{
		"header_key":      "headerField",
		"header_value":    "headerValue",
		attrs.LogFileName: filepath.Base(temp.Name()),
	})
}

func TestDeleteAfterRead_SkipPartials(t *testing.T) {
	shortFileLine := "short file line"
	longFileLines := 100000

	require.NoError(t, featuregate.GlobalRegistry().Set(allowFileDeletion.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(allowFileDeletion.ID(), false))
	}()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.DeleteAfterRead = true
	sink := emittest.NewSink(emittest.WithCallBuffer(longFileLines + 1))
	operator := testManagerWithSink(t, cfg, sink)
	operator.persister = testutil.NewUnscopedMockPersister()

	shortFile := filetest.OpenTemp(t, tempDir)
	_, err := shortFile.WriteString(shortFileLine + "\n")
	require.NoError(t, err)
	require.NoError(t, shortFile.Close())

	longFile := filetest.OpenTemp(t, tempDir)
	for line := 0; line < longFileLines; line++ {
		_, err := longFile.WriteString(string(filetest.TokenWithLength(100)) + "\n")
		require.NoError(t, err)
	}
	require.NoError(t, longFile.Close())

	// Verify we have no checkpointed files
	require.Equal(t, 0, operator.tracker.TotalReaders())

	// Wait until the only line in the short file and
	// at least one line from the long file have been consumed
	var shortOne, longOne bool
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		operator.poll(ctx)
	}()

	for !shortOne || !longOne {
		if token := sink.NextToken(t); string(token) == shortFileLine {
			shortOne = true
		} else {
			longOne = true
		}
	}

	// Short file was fully consumed and should eventually be deleted.
	// Enforce assertion before canceling because EOF is not necessarily detected
	// immediately when the token is emitted. An additional scan may be necessary.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoFileExists(c, shortFile.Name())
	}, 100*time.Millisecond, time.Millisecond)

	// Stop consuming before long file has been fully consumed
	cancel()
	wg.Wait()

	// Long file was partially consumed and should NOT have been deleted.
	require.FileExists(t, longFile.Name())
}

func TestHeaderPersistance(t *testing.T) {
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg = cfg.withHeader("^#", "(?P<header_key>[A-z]+): (?P<header_value>[A-z]+)")

	op1, sink1 := testManager(t, cfg)

	// Create a file, then start
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "#headerField: headerValue\nlog line\n")

	persister := testutil.NewUnscopedMockPersister()

	require.NoError(t, op1.Start(persister))
	sink1.ExpectCall(t, []byte("log line"), map[string]any{
		"header_key":      "headerField",
		"header_value":    "headerValue",
		attrs.LogFileName: filepath.Base(temp.Name()),
	})
	require.NoError(t, op1.Stop())

	filetest.WriteString(t, temp, "log line 2\n")

	op2, sink2 := testManager(t, cfg)

	require.NoError(t, op2.Start(persister))
	sink2.ExpectCall(t, []byte("log line 2"), map[string]any{
		"header_key":      "headerField",
		"header_value":    "headerValue",
		attrs.LogFileName: filepath.Base(temp.Name()),
	})
	require.NoError(t, op2.Stop())
}

func TestHeaderPersistanceInHeader(t *testing.T) {
	tempDir := t.TempDir()
	cfg1 := NewConfig().includeDir(tempDir)
	cfg1.StartAt = "beginning"
	cfg1 = cfg1.withHeader(`^\|`, "headerField1: (?P<header_value_1>[A-z0-9]+)")

	op1, _ := testManager(t, cfg1)

	// Create a file, then start
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "|headerField1: headerValue1\n")

	persister := testutil.NewUnscopedMockPersister()

	// Start and stop the operator, ensuring that at least one poll cycle occurs in between
	require.NoError(t, op1.Start(persister))
	time.Sleep(2 * cfg1.PollInterval)
	require.NoError(t, op1.Stop())

	filetest.WriteString(t, temp, "|headerField2: headerValue2\nlog line\n")

	cfg2 := NewConfig().includeDir(tempDir)
	cfg2.StartAt = "beginning"
	cfg2 = cfg2.withHeader(`^\|`, "headerField2: (?P<header_value_2>[A-z0-9]+)")

	op2, sink := testManager(t, cfg2)

	require.NoError(t, op2.Start(persister))
	sink.ExpectCall(t, []byte("log line"), map[string]any{
		"header_value_1":  "headerValue1",
		"header_value_2":  "headerValue2",
		attrs.LogFileName: filepath.Base(temp.Name()),
	})
	require.NoError(t, op2.Stop())
}

func TestStalePartialFingerprintDiscarded(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.FingerprintSize = 18
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	// Both of they will be include
	file1 := filetest.OpenTempWithPattern(t, tempDir, "*.log1")
	file2 := filetest.OpenTempWithPattern(t, tempDir, "*.log2")

	// Two same fingerprint file , and smaller than config size
	content := "aaaaaaaaaaa"
	filetest.WriteString(t, file1, content+"\n")
	filetest.WriteString(t, file2, content+"\n")
	operator.poll(context.Background())
	// one file will be exclude, ingest only one content
	sink.ExpectToken(t, []byte(content))
	sink.ExpectNoCalls(t)
	operator.wg.Wait()
	if runtime.GOOS != "windows" {
		// On windows, we never keep files in previousPollFiles, so we don't expect to see them here
		require.Len(t, operator.tracker.PreviousPollFiles(), 1)
	}

	// keep append data to file1 and file2
	newContent := "bbbbbbbbbbbb"
	newContent1 := "ddd"
	filetest.WriteString(t, file1, newContent1+"\n")
	filetest.WriteString(t, file2, newContent+"\n")
	operator.poll(context.Background())
	// We should have updated the offset for one of the files, so the second file should now
	// be ingested from the beginning
	sink.ExpectTokens(t, []byte(content), []byte(newContent1), []byte(newContent))
	operator.wg.Wait()
}

func TestWindowsFilesClosedImmediately(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "testlog\n")
	require.NoError(t, temp.Close())

	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog"))

	// On Windows, poll should close the file after reading it. We can test this by trying to move it.
	require.NoError(t, os.Rename(temp.Name(), temp.Name()+"_renamed"))
}

func TestDelayedDisambiguation(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.FingerprintSize = 18
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	// Two identical files, smaller than fingerprint size
	file1 := filetest.OpenTempWithPattern(t, tempDir, "*.log1")
	file2 := filetest.OpenTempWithPattern(t, tempDir, "*.log2")

	sameContent := "aaaaaaaaaaa"
	filetest.WriteString(t, file1, sameContent+"\n")
	filetest.WriteString(t, file2, sameContent+"\n")
	operator.poll(context.Background())

	token, attributes := sink.NextCall(t)
	require.Equal(t, []byte(sameContent), token)
	sink.ExpectNoCallsUntil(t, 100*time.Millisecond)
	operator.wg.Wait()

	// Append different data
	newContent1 := "more content in file 1 only"
	newContent2 := "different content in file 2"
	filetest.WriteString(t, file1, newContent1+"\n")
	filetest.WriteString(t, file2, newContent2+"\n")
	operator.poll(context.Background())

	var sameTokenOtherFile emit.Token
	if attributes[attrs.LogFileName].(string) == filepath.Base(file1.Name()) {
		sameTokenOtherFile = emit.Token{Body: []byte(sameContent), Attributes: map[string]any{attrs.LogFileName: filepath.Base(file2.Name())}}
	} else {
		sameTokenOtherFile = emit.Token{Body: []byte(sameContent), Attributes: map[string]any{attrs.LogFileName: filepath.Base(file1.Name())}}
	}
	newFromFile1 := emit.Token{Body: []byte(newContent1), Attributes: map[string]any{attrs.LogFileName: filepath.Base(file1.Name())}}
	newFromFile2 := emit.Token{Body: []byte(newContent2), Attributes: map[string]any{attrs.LogFileName: filepath.Base(file2.Name())}}
	sink.ExpectCalls(t, sameTokenOtherFile, newFromFile1, newFromFile2)
}

func TestNoLostPartial(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.FingerprintSize = 18
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	// Two same fingerprint file , and smaller than config size
	file1 := filetest.OpenTempWithPattern(t, tempDir, "*.log1")
	file2 := filetest.OpenTempWithPattern(t, tempDir, "*.log2")

	sameContent := "aaaaaaaaaaa"
	filetest.WriteString(t, file1, sameContent+"\n")
	filetest.WriteString(t, file2, sameContent+"\n")
	operator.poll(context.Background())

	token, attributes := sink.NextCall(t)
	require.Equal(t, []byte(sameContent), token)
	sink.ExpectNoCallsUntil(t, 100*time.Millisecond)
	operator.wg.Wait()

	newContent1 := "additional content in file 1 only"
	filetest.WriteString(t, file1, newContent1+"\n")

	var otherFileName string
	if attributes[attrs.LogFileName].(string) == filepath.Base(file1.Name()) {
		otherFileName = filepath.Base(file2.Name())
	} else {
		otherFileName = filepath.Base(file1.Name())
	}

	var foundSameFromOtherFile, foundNewFromFileOne bool
	require.Eventually(t, func() bool {
		operator.poll(context.Background())
		defer operator.wg.Wait()

		token, attributes = sink.NextCall(t)
		switch {
		case string(token) == sameContent && attributes[attrs.LogFileName].(string) == otherFileName:
			foundSameFromOtherFile = true
		case string(token) == newContent1 && attributes[attrs.LogFileName].(string) == filepath.Base(file1.Name()):
			foundNewFromFileOne = true
		default:
			t.Errorf("unexpected token from file %q: %s", filepath.Base(attributes[attrs.LogFileName].(string)), token)
		}
		return foundSameFromOtherFile && foundNewFromFileOne
	}, time.Second, 100*time.Millisecond)
}

func TestNoTracking(t *testing.T) {
	testCases := []struct {
		testName     string
		noTracking   bool
		expectReplay bool
	}{
		{"tracking_enabled", false, false},
		{"tracking_disabled", true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			tempDir := t.TempDir()
			cfg := NewConfig().includeDir(tempDir)
			cfg.StartAt = "beginning"
			cfg.PollInterval = 1000 * time.Hour // We control the polling within the test.

			opts := make([]Option, 0)
			if tc.noTracking {
				opts = append(opts, WithNoTracking())
			}
			operator, sink := testManager(t, cfg, opts...)

			temp := filetest.OpenTemp(t, tempDir)
			filetest.WriteString(t, temp, " testlog1 \n")

			require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			operator.poll(context.Background())
			sink.ExpectToken(t, []byte("testlog1"))

			// Poll again and see if the file is replayed.
			operator.poll(context.Background())
			if tc.expectReplay {
				sink.ExpectToken(t, []byte("testlog1"))
			} else {
				sink.ExpectNoCalls(t)
			}
		})
	}
}

func symlinkTestCreateLogFile(t *testing.T, tempDir string, fileIdx, numLogLines int) (tokens [][]byte) {
	logFilePath := fmt.Sprintf("%s/%d.log", tempDir, fileIdx)
	temp1 := filetest.OpenFile(t, logFilePath)
	for i := 0; i < numLogLines; i++ {
		msg := fmt.Sprintf("[fileIdx %2d] This is a simple log line with the number %3d", fileIdx, i)
		filetest.WriteString(t, temp1, msg+"\n")
		tokens = append(tokens, []byte(msg))
	}
	temp1.Close()
	return tokens
}

// TestReadGzipCompressedLogsFromBeginning tests that, when starting from beginning of a gzip compressed file, we
// read all the lines that are already there
func TestReadGzipCompressedLogsFromBeginning(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir).withGzip()
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

	// Create a file, then start
	temp := filetest.OpenTempWithPattern(t, tempDir, "*.gz")
	writer := gzip.NewWriter(temp)

	_, err := writer.Write([]byte("testlog1\ntestlog2\n"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte("testlog1"))
	sink.ExpectToken(t, []byte("testlog2"))
}

// TestReadGzipCompressedLogsFromEnd tests that, when starting at the end of a gzip compressed file, we
// read all the lines that are added afterward
func TestReadGzipCompressedLogsFromEnd(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir).withGzip()
	cfg.StartAt = "end"
	operator, sink := testManager(t, cfg)

	// Create a file, then start
	temp := filetest.OpenTempWithPattern(t, tempDir, "*.gz")

	appendToLog := func(t *testing.T, content string) {
		writer := gzip.NewWriter(temp)
		_, err := writer.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, writer.Close())
	}

	appendToLog(t, "testlog1\ntestlog2\n")

	// poll for the first time - this should not lead to emitted
	// logs as those were already in the existing file
	operator.poll(context.TODO())

	// append new content to the log and poll again - this should be picked up
	appendToLog(t, "testlog3\n")
	operator.poll(context.TODO())
	sink.ExpectToken(t, []byte("testlog3"))

	// do another iteration to verify correct setting of compressed reader offset
	appendToLog(t, "testlog4\n")
	operator.poll(context.TODO())
	sink.ExpectToken(t, []byte("testlog4"))
}
