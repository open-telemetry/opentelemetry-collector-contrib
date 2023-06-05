// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileconsumer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestCleanStop(t *testing.T) {
	t.Parallel()
	t.Skip(`Skipping due to goroutine leak in opencensus.
See this issue for details: https://github.com/census-instrumentation/opencensus-go/issues/1191#issuecomment-610440163`)
	// defer goleak.VerifyNone(t)

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, _ := buildTestManager(t, cfg)

	_ = openTemp(t, tempDir)
	err := operator.Start(testutil.NewMockPersister("test"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, operator.Stop())
	}()
}

// AddFields tests that the `log.file.name` and `log.file.path` fields are included
// when IncludeFileName and IncludeFilePath are set to true
func TestAddFileFields(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.IncludeFileName = true
	cfg.IncludeFilePath = true
	operator, emitCalls := buildTestManager(t, cfg)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	emitCall := waitForEmit(t, emitCalls)
	require.Equal(t, filepath.Base(temp.Name()), emitCall.attrs.Name)
	require.Equal(t, temp.Name(), emitCall.attrs.Path)
}

// AddFileResolvedFields tests that the `log.file.name_resolved` and `log.file.path_resolved` fields are included
// when IncludeFileNameResolved and IncludeFilePathResolved are set to true
func TestAddFileResolvedFields(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Windows symlinks usage disabled for now. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21088")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.IncludeFileName = true
	cfg.IncludeFilePath = true
	cfg.IncludeFileNameResolved = true
	cfg.IncludeFilePathResolved = true
	operator, emitCalls := buildTestManager(t, cfg)

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
	real, err := filepath.EvalSymlinks(file.Name())
	require.NoError(t, err)
	resolved, err := filepath.Abs(real)
	require.NoError(t, err)

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	emitCall := waitForEmit(t, emitCalls)
	require.Equal(t, filepath.Base(symLinkPath), emitCall.attrs.Name)
	require.Equal(t, symLinkPath, emitCall.attrs.Path)
	require.Equal(t, filepath.Base(resolved), emitCall.attrs.NameResolved)
	require.Equal(t, resolved, emitCall.attrs.PathResolved)
}

// AddFileResolvedFields tests that the `log.file.name_resolved` and `log.file.path_resolved` fields are included
// when IncludeFileNameResolved and IncludeFilePathResolved are set to true and underlaying symlink change
// Scenario:
// monitored file (symlink) -> middleSymlink -> file_1
// monitored file (symlink) -> middleSymlink -> file_2
func TestAddFileResolvedFieldsWithChangeOfSymlinkTarget(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Windows symlinks usage disabled for now. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21088")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.IncludeFileName = true
	cfg.IncludeFilePath = true
	cfg.IncludeFileNameResolved = true
	cfg.IncludeFilePathResolved = true
	operator, emitCalls := buildTestManager(t, cfg)

	// Create temp dir with log file
	dir := t.TempDir()

	file1, err := os.CreateTemp(dir, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file1.Close())
	})

	file2, err := os.CreateTemp(dir, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file2.Close())
	})

	// Resolve paths
	real1, err := filepath.EvalSymlinks(file1.Name())
	require.NoError(t, err)
	resolved1, err := filepath.Abs(real1)
	require.NoError(t, err)

	real2, err := filepath.EvalSymlinks(file2.Name())
	require.NoError(t, err)
	resolved2, err := filepath.Abs(real2)
	require.NoError(t, err)

	// Create symbolic link in monitored directory
	// symLinkPath(target of file input) -> middleSymLinkPath -> file1
	middleSymLinkPath := filepath.Join(dir, "symlink")
	symLinkPath := filepath.Join(tempDir, "symlink")
	err = os.Symlink(file1.Name(), middleSymLinkPath)
	require.NoError(t, err)
	err = os.Symlink(middleSymLinkPath, symLinkPath)
	require.NoError(t, err)

	// Populate data
	writeString(t, file1, "testlog\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	emitCall := waitForEmit(t, emitCalls)
	require.Equal(t, filepath.Base(symLinkPath), emitCall.attrs.Name)
	require.Equal(t, symLinkPath, emitCall.attrs.Path)
	require.Equal(t, filepath.Base(resolved1), emitCall.attrs.NameResolved)
	require.Equal(t, resolved1, emitCall.attrs.PathResolved)

	// Change middleSymLink to point to file2
	err = os.Remove(middleSymLinkPath)
	require.NoError(t, err)
	err = os.Symlink(file2.Name(), middleSymLinkPath)
	require.NoError(t, err)

	// Populate data (different content due to fingerprint)
	writeString(t, file2, "testlog2\n")

	emitCall = waitForEmit(t, emitCalls)
	require.Equal(t, filepath.Base(symLinkPath), emitCall.attrs.Name)
	require.Equal(t, symLinkPath, emitCall.attrs.Path)
	require.Equal(t, filepath.Base(resolved2), emitCall.attrs.NameResolved)
	require.Equal(t, resolved2, emitCall.attrs.PathResolved)
}

// ReadExistingLogs tests that, when starting from beginning, we
// read all the lines that are already there
func TestReadExistingLogs(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte("testlog2"))
}

// TestReadUsingNopEncoding tests when nop encoding is set, that the splitfunction returns all bytes unchanged.
func TestReadUsingNopEncoding(t *testing.T) {
	tcs := []struct {
		testName string
		input    []byte
		test     func(*testing.T, chan *emitParams)
	}{
		{
			"simple",
			[]byte("testlog1"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog1"))
			},
		},
		{
			"longer than maxlogsize",
			[]byte("testlog1testlog2testlog3"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog1"))
				waitForToken(t, c, []byte("testlog2"))
				waitForToken(t, c, []byte("testlog3"))
			},
		},
		{
			"doesn't hit max log size before eof",
			[]byte("testlog1testlog2test"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog1"))
				waitForToken(t, c, []byte("testlog2"))
				waitForToken(t, c, []byte("test"))
			},
		},
		{
			"special characters",
			[]byte("testlog1\n\ttestlog2\n\t"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog1"))
				waitForToken(t, c, []byte("\n\ttestlo"))
				waitForToken(t, c, []byte("g2\n\t"))
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
			cfg.Splitter.EncodingConfig.Encoding = "nop"
			operator, emitCalls := buildTestManager(t, cfg)

			// Create a file, then start
			temp := openTemp(t, tempDir)
			bytesWritten, err := temp.Write(tc.input)
			require.Greater(t, bytesWritten, 0)
			require.NoError(t, err)
			require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			tc.test(t, emitCalls)
		})
	}
}

func TestNopEncodingDifferentLogSizes(t *testing.T) {
	tcs := []struct {
		testName   string
		input      []byte
		test       func(*testing.T, chan *emitParams)
		maxLogSize helper.ByteSize
	}{
		{
			"same size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog1"))
			},
			8,
		},
		{
			"massive log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog1"))
			},
			8000000,
		},
		{
			"slightly larger log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog1"))
			},
			9,
		},
		{
			"slightly smaller log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("testlog"))
				waitForToken(t, c, []byte("1"))
			},
			7,
		},
		{
			"tiny log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *emitParams) {
				waitForToken(t, c, []byte("t"))
				waitForToken(t, c, []byte("e"))
				waitForToken(t, c, []byte("s"))
				waitForToken(t, c, []byte("t"))
				waitForToken(t, c, []byte("l"))
				waitForToken(t, c, []byte("o"))
				waitForToken(t, c, []byte("g"))
				waitForToken(t, c, []byte("1"))
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
			cfg.Splitter.EncodingConfig.Encoding = "nop"
			operator, emitCalls := buildTestManager(t, cfg)

			// Create a file, then start
			temp := openTemp(t, tempDir)
			bytesWritten, err := temp.Write(tc.input)
			require.Greater(t, bytesWritten, 0)
			require.NoError(t, err)
			require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			tc.test(t, emitCalls)
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
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	// Poll once so we know this isn't a new file
	operator.poll(context.Background())
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// Create a new file
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog\n")

	// Poll a second time after the file has been created
	operator.poll(context.Background())

	// Expect the message to come through
	waitForToken(t, emitCalls, []byte("testlog"))
}

// ReadExistingAndNewLogs tests that, on startup, if start_at
// is set to `beginning`, we read the logs that are there, and
// we read any additional logs that are written after startup
func TestReadExistingAndNewLogs(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	// Start with a file with an entry in it, and expect that entry
	// to come through when we poll for the first time
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n")
	operator.poll(context.Background())
	waitForToken(t, emitCalls, []byte("testlog1"))

	// Write a second entry, and expect that entry to come through
	// as well
	writeString(t, temp, "testlog2\n")
	operator.poll(context.Background())
	waitForToken(t, emitCalls, []byte("testlog2"))
}

// StartAtEnd tests that when `start_at` is configured to `end`,
// we don't read any entries that were in the file before startup
func TestStartAtEnd(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n")

	// Expect no entries on the first poll
	operator.poll(context.Background())
	expectNoTokens(t, emitCalls)

	// Expect any new entries after the first poll
	writeString(t, temp, "testlog2\n")
	operator.poll(context.Background())
	waitForToken(t, emitCalls, []byte("testlog2"))
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
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	operator.poll(context.Background())
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte("testlog2"))
}

// NoNewline tests that an entry will still be sent eventually
// even if the file doesn't end in a newline
func TestNoNewline(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.Splitter = helper.NewSplitterConfig()
	cfg.Splitter.Flusher.Period = time.Nanosecond
	operator, emitCalls := buildTestManager(t, cfg)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte("testlog2"))
}

// TestEmptyLine tests that the any empty lines are consumed
func TestEmptyLine(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte(""))
	waitForToken(t, emitCalls, []byte("testlog2"))
}

// TestMultipleEmpty tests that multiple empty lines
// can be consumed without the operator becoming stuck
func TestMultipleEmpty(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "\n\ntestlog1\n\n\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte(""))
	waitForToken(t, emitCalls, []byte(""))
	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte(""))
	waitForToken(t, emitCalls, []byte(""))
	waitForToken(t, emitCalls, []byte("testlog2"))
	expectNoTokensUntil(t, emitCalls, time.Second)
}

// TestLeadingEmpty tests that the the operator handles a leading
// newline, and does not read the file multiple times
func TestLeadingEmpty(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "\ntestlog1\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte(""))
	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte("testlog2"))
	expectNoTokensUntil(t, emitCalls, time.Second)
}

// SplitWrite tests a line written in two writes
// close together still is read as a single entry
func TestSplitWrite(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1")

	operator.poll(context.Background())

	writeString(t, temp, "testlog2\n")

	operator.poll(context.Background())
	waitForToken(t, emitCalls, []byte("testlog1testlog2"))
}

func TestIgnoreEmptyFiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	temp := openTemp(t, tempDir)
	temp2 := openTemp(t, tempDir)
	temp3 := openTemp(t, tempDir)
	temp4 := openTemp(t, tempDir)

	writeString(t, temp, "testlog1\n")
	writeString(t, temp3, "testlog2\n")
	operator.poll(context.Background())

	waitForTokens(t, emitCalls, [][]byte{[]byte("testlog1"), []byte("testlog2")})

	writeString(t, temp2, "testlog3\n")
	writeString(t, temp4, "testlog4\n")
	operator.poll(context.Background())

	waitForTokens(t, emitCalls, [][]byte{[]byte("testlog3"), []byte("testlog4")})
}

func TestDecodeBufferIsResized(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	temp := openTemp(t, tempDir)
	expected := tokenWithLength(1<<12 + 1)
	writeString(t, temp, string(expected)+"\n")

	waitForToken(t, emitCalls, expected)
}

func TestMultiFileSimple(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	temp1 := openTemp(t, tempDir)
	temp2 := openTemp(t, tempDir)

	writeString(t, temp1, "testlog1\n")
	writeString(t, temp2, "testlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForTokens(t, emitCalls, [][]byte{[]byte("testlog1"), []byte("testlog2")})
}

func TestMultiFileParallel_PreloadedFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

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
		temp := openTemp(t, tempDir)
		wg.Add(1)
		go func(tf *os.File, f int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				writeString(t, tf, getMessage(f, j)+"\n")
			}
		}(temp, i)
	}

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForTokens(t, emitCalls, expected)
	wg.Wait()
}

func TestMultiFileParallel_LiveFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	numFiles := 10
	numMessages := 100

	expected := make([][]byte, 0, numFiles*numMessages)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			expected = append(expected, []byte(getMessage(i, j)))
		}
	}

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	temps := make([]*os.File, 0, numFiles)
	for i := 0; i < numFiles; i++ {
		temps = append(temps, openTemp(t, tempDir))
	}

	var wg sync.WaitGroup
	for i, temp := range temps {
		wg.Add(1)
		go func(tf *os.File, f int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				writeString(t, tf, getMessage(f, j)+"\n")
			}
		}(temp, i)
	}

	waitForTokens(t, emitCalls, expected)
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

			persister := testutil.NewMockPersister("test")

			logFile := openTemp(t, tempDir)

			before1stRun := tokenWithLength(tc.lineLength)
			during1stRun := tokenWithLength(tc.lineLength)
			duringRestart := tokenWithLength(tc.lineLength)
			during2ndRun := tokenWithLength(tc.lineLength)

			operatorOne, emitCallsOne := buildTestManager(t, cfg)
			writeString(t, logFile, string(before1stRun)+"\n")
			require.NoError(t, operatorOne.Start(persister))
			if tc.startAt == "beginning" {
				waitForToken(t, emitCallsOne, before1stRun)
			} else {
				expectNoTokensUntil(t, emitCallsOne, 500*time.Millisecond)
			}
			writeString(t, logFile, string(during1stRun)+"\n")
			waitForToken(t, emitCallsOne, during1stRun)
			require.NoError(t, operatorOne.Stop())

			writeString(t, logFile, string(duringRestart)+"\n")

			operatorTwo, emitCallsTwo := buildTestManager(t, cfg)
			require.NoError(t, operatorTwo.Start(persister))
			waitForToken(t, emitCallsTwo, duringRestart)
			writeString(t, logFile, string(during2ndRun)+"\n")
			waitForToken(t, emitCallsTwo, during2ndRun)
			require.NoError(t, operatorTwo.Stop())
		})
	}
}

func TestManyLogsDelivered(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	count := 1000
	expectedTokens := make([]string, 0, count)
	for i := 0; i < count; i++ {
		expectedTokens = append(expectedTokens, strconv.Itoa(i))
	}

	// Start the operator
	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// Write lots of logs
	temp := openTemp(t, tempDir)
	for _, message := range expectedTokens {
		_, err := temp.WriteString(message + "\n")
		require.NoError(t, err)
	}

	// Expect each of them to come through once
	for _, message := range expectedTokens {
		waitForToken(t, emitCalls, []byte(message))
	}
	expectNoTokens(t, emitCalls)
}

func TestFileBatching(t *testing.T) {
	t.Parallel()

	files := 100
	linesPerFile := 10
	maxConcurrentFiles := 20
	maxBatchFiles := maxConcurrentFiles / 2
	// Explicitly setting maxBatches to ensure a value of 0 does not enforce a limit
	maxBatches := 0

	expectedBatches := files / maxBatchFiles // assumes no remainder

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.MaxConcurrentFiles = maxConcurrentFiles
	cfg.MaxBatches = maxBatches
	emitCalls := make(chan *emitParams, files*linesPerFile)
	operator := buildTestManagerWithEmit(t, cfg, emitCalls)
	operator.persister = testutil.NewMockPersister("test")

	core, observedLogs := observer.New(zap.DebugLevel)
	operator.SugaredLogger = zap.New(core).Sugar()

	temps := make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, openTemp(t, tempDir))
	}

	// Write logs to each file
	expectedTokens := make([][]byte, 0, files*linesPerFile)
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", tokenWithLength(100), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
			expectedTokens = append(expectedTokens, []byte(message))
		}
	}

	// Poll and wait for all lines
	operator.poll(context.Background())
	actualTokens := make([][]byte, 0, files*linesPerFile)
	actualTokens = append(actualTokens, waitForNTokens(t, emitCalls, len(expectedTokens))...)
	require.ElementsMatch(t, expectedTokens, actualTokens)

	// During the first poll, we expect one log per batch and one log per file
	require.Equal(t, files+expectedBatches, observedLogs.Len())
	logNum := 0
	for b := 0; b < expectedBatches; b++ {
		log := observedLogs.All()[logNum]
		require.Equal(t, "Consuming files", log.Message)
		require.Equal(t, zapcore.DebugLevel, log.Level)
		logNum++

		for f := 0; f < maxBatchFiles; f++ {
			log = observedLogs.All()[logNum]
			require.Equal(t, "Started watching file", log.Message)
			require.Equal(t, zapcore.InfoLevel, log.Level)
			logNum++
		}
	}

	// Write more logs to each file so we can validate that all files are still known
	expectedTokens = make([][]byte, 0, files*linesPerFile)
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", tokenWithLength(20), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
			expectedTokens = append(expectedTokens, []byte(message))
		}
	}

	// Poll again and wait for all new lines
	operator.poll(context.Background())
	actualTokens = make([][]byte, 0, files*linesPerFile)
	actualTokens = append(actualTokens, waitForNTokens(t, emitCalls, len(expectedTokens))...)
	require.ElementsMatch(t, expectedTokens, actualTokens)

	// During the second poll, we only expect one log per batch
	require.Equal(t, files+expectedBatches*2, observedLogs.Len())
	for b := logNum; b < observedLogs.Len(); b++ {
		log := observedLogs.All()[logNum]
		require.Equal(t, "Consuming files", log.Message)
		require.Equal(t, zapcore.DebugLevel, log.Level)
		logNum++
	}
}

func TestFileReader_FingerprintUpdated(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	temp := openTemp(t, tempDir)
	tempCopy := openFile(t, temp.Name())
	fp, err := operator.readerFactory.newFingerprint(temp)
	require.NoError(t, err)

	reader, err := operator.readerFactory.newReader(tempCopy, fp)
	require.NoError(t, err)
	defer reader.Close()

	writeString(t, temp, "testlog1\n")
	reader.ReadToEnd(context.Background())
	waitForToken(t, emitCalls, []byte("testlog1"))
	require.Equal(t, []byte("testlog1\n"), reader.Fingerprint.FirstBytes)
}

// Test that a fingerprint:
// - Starts empty
// - Updates as a file is read
// - Stops updating when the max fingerprint size is reached
// - Stops exactly at max fingerprint size, regardless of content
func TestFingerprintGrowsAndStops(t *testing.T) {
	t.Parallel()

	// Use a number with many factors.
	// Sometimes fingerprint length will align with
	// the end of a line, sometimes not. Test both.
	maxFP := 360

	// Use prime numbers to ensure variation in
	// whether or not they are factors of maxFP
	lineLens := []int{3, 5, 7, 11, 13, 17, 19, 23, 27}

	for _, lineLen := range lineLens {
		t.Run(fmt.Sprintf("%d", lineLen), func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			cfg := NewConfig().includeDir(tempDir)
			cfg.StartAt = "beginning"
			cfg.FingerprintSize = helper.ByteSize(maxFP)
			operator, _ := buildTestManager(t, cfg)

			temp := openTemp(t, tempDir)
			tempCopy := openFile(t, temp.Name())
			fp, err := operator.readerFactory.newFingerprint(temp)
			require.NoError(t, err)
			require.Equal(t, []byte(""), fp.FirstBytes)

			reader, err := operator.readerFactory.newReader(tempCopy, fp)
			require.NoError(t, err)
			defer reader.Close()

			// keep track of what has been written to the file
			var fileContent []byte

			// keep track of expected fingerprint size
			expectedFP := 0

			// Write lines until file is much larger than the length of the fingerprint
			for len(fileContent) < 2*maxFP {
				expectedFP += lineLen
				if expectedFP > maxFP {
					expectedFP = maxFP
				}

				line := string(tokenWithLength(lineLen-1)) + "\n"
				fileContent = append(fileContent, []byte(line)...)

				writeString(t, temp, line)
				reader.ReadToEnd(context.Background())
				require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)
			}
		})
	}
}

// This is same test like TestFingerprintGrowsAndStops, but with additional check for fingerprint size check
// Test that a fingerprint:
// - Starts empty
// - Updates as a file is read
// - Stops updating when the max fingerprint size is reached
// - Stops exactly at max fingerprint size, regardless of content
// - Do not change size after fingerprint configuration change
func TestFingerprintChangeSize(t *testing.T) {
	t.Parallel()

	// Use a number with many factors.
	// Sometimes fingerprint length will align with
	// the end of a line, sometimes not. Test both.
	maxFP := 360

	// Use prime numbers to ensure variation in
	// whether or not they are factors of maxFP
	lineLens := []int{3, 5, 7, 11, 13, 17, 19, 23, 27}

	for _, lineLen := range lineLens {
		t.Run(fmt.Sprintf("%d", lineLen), func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			cfg := NewConfig().includeDir(tempDir)
			cfg.StartAt = "beginning"
			cfg.FingerprintSize = helper.ByteSize(maxFP)
			operator, _ := buildTestManager(t, cfg)

			temp := openTemp(t, tempDir)
			tempCopy := openFile(t, temp.Name())
			fp, err := operator.readerFactory.newFingerprint(temp)
			require.NoError(t, err)
			require.Equal(t, []byte(""), fp.FirstBytes)

			reader, err := operator.readerFactory.newReader(tempCopy, fp)
			require.NoError(t, err)
			defer reader.Close()

			// keep track of what has been written to the file
			var fileContent []byte

			// keep track of expected fingerprint size
			expectedFP := 0

			// Write lines until file is much larger than the length of the fingerprint
			for len(fileContent) < 2*maxFP {
				expectedFP += lineLen
				if expectedFP > maxFP {
					expectedFP = maxFP
				}

				line := string(tokenWithLength(lineLen-1)) + "\n"
				fileContent = append(fileContent, []byte(line)...)

				writeString(t, temp, line)
				reader.ReadToEnd(context.Background())
				require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)
			}

			// Test fingerprint change
			// Change fingerprint and try to read file again
			// We do not expect fingerprint change
			// We test both increasing and decreasing fingerprint size
			reader.readerConfig.fingerprintSize = maxFP * (lineLen / 3)
			line := string(tokenWithLength(lineLen-1)) + "\n"
			fileContent = append(fileContent, []byte(line)...)

			writeString(t, temp, line)
			reader.ReadToEnd(context.Background())
			require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)

			reader.readerConfig.fingerprintSize = maxFP / 2
			line = string(tokenWithLength(lineLen-1)) + "\n"
			fileContent = append(fileContent, []byte(line)...)

			writeString(t, temp, line)
			reader.ReadToEnd(context.Background())
			require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)
		})
	}
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
			"",
			[][]byte{{0xc5}},
		},
		{
			"InvalidUTFReplacement",
			[]byte{0xc5, '\n'},
			"utf8",
			[][]byte{{0xef, 0xbf, 0xbd}},
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
			cfg.Splitter.EncodingConfig = helper.EncodingConfig{Encoding: tc.encoding}
			operator, emitCalls := buildTestManager(t, cfg)

			// Populate the file
			temp := openTemp(t, tempDir)
			_, err := temp.Write(tc.contents)
			require.NoError(t, err)

			require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			waitForTokens(t, emitCalls, tc.expected)
		})
	}
}

func TestDeleteAfterRead(t *testing.T) {
	t.Parallel()

	files := 10
	linesPerFile := 10
	totalLines := files * linesPerFile

	expectedTokens := make([][]byte, 0, totalLines)
	actualTokens := make([][]byte, 0, totalLines)

	tempDir := t.TempDir()
	temps := make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, openTemp(t, tempDir))
	}

	// Write logs to each file
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			line := tokenWithLength(100)
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
	emitCalls := make(chan *emitParams, totalLines)
	operator := buildTestManagerWithEmit(t, cfg, emitCalls)

	operator.poll(context.Background())
	actualTokens = append(actualTokens, waitForNTokens(t, emitCalls, totalLines)...)

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

	expectedBatches := maxBatches
	expectedMaxFilesPerPoll := maxBatches * maxBatchFiles

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.MaxConcurrentFiles = maxConcurrentFiles
	cfg.MaxBatches = maxBatches
	emitCalls := make(chan *emitParams, files*linesPerFile)
	operator := buildTestManagerWithEmit(t, cfg, emitCalls)
	operator.persister = testutil.NewMockPersister("test")

	core, observedLogs := observer.New(zap.DebugLevel)
	operator.SugaredLogger = zap.New(core).Sugar()

	temps := make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, openTemp(t, tempDir))
	}

	// Write logs to each file
	numExpectedTokens := expectedMaxFilesPerPoll * linesPerFile
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", tokenWithLength(100), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
		}
	}

	// Poll and wait for all lines
	operator.poll(context.Background())
	actualTokens := make([][]byte, 0, numExpectedTokens)
	actualTokens = append(actualTokens, waitForNTokens(t, emitCalls, numExpectedTokens)...)
	require.Len(t, actualTokens, numExpectedTokens)

	// During the first poll, we expect one log per batch and one log per file
	require.Equal(t, expectedMaxFilesPerPoll+expectedBatches, observedLogs.Len())
	logNum := 0
	for b := 0; b < expectedBatches; b++ {
		log := observedLogs.All()[logNum]
		require.Equal(t, "Consuming files", log.Message)
		require.Equal(t, zapcore.DebugLevel, log.Level)
		logNum++

		for f := 0; f < maxBatchFiles; f++ {
			log = observedLogs.All()[logNum]
			require.Equal(t, "Started watching file", log.Message)
			require.Equal(t, zapcore.InfoLevel, log.Level)
			logNum++
		}
	}

	// Write more logs to each file so we can validate that all files are still known
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", tokenWithLength(20), i, j)
			_, err := temp.WriteString(message + "\n")
			require.NoError(t, err)
		}
	}

	// Poll again and wait for all new lines
	operator.poll(context.Background())
	actualTokens = make([][]byte, 0, numExpectedTokens)
	actualTokens = append(actualTokens, waitForNTokens(t, emitCalls, numExpectedTokens)...)
	require.Len(t, actualTokens, numExpectedTokens)

	// During the second poll, we only expect one log per batch
	require.Equal(t, expectedMaxFilesPerPoll+expectedBatches*2, observedLogs.Len())
	for b := logNum; b < observedLogs.Len(); b++ {
		log := observedLogs.All()[logNum]
		require.Equal(t, "Consuming files", log.Message)
		require.Equal(t, zapcore.DebugLevel, log.Level)
		logNum++
	}
}

// TestReadExistingLogsWithHeader tests that, when starting from beginning, we
// read all the lines that are already there, and parses the headers
func TestReadExistingLogsWithHeader(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), false))
	})

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg = cfg.withHeader("^#", "(?P<header_key>[A-z]+): (?P<header_value>[A-z]+)")

	operator, emitCalls := buildTestManager(t, cfg)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "#headerField: headerValue\ntestlog\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForTokenHeaderAttributes(t, emitCalls, []byte("testlog"), map[string]any{
		"header_key":   "headerField",
		"header_value": "headerValue",
	})
}

func TestDeleteAfterRead_SkipPartials(t *testing.T) {
	bytesPerLine := 100
	shortFileLine := tokenWithLength(bytesPerLine - 1)
	longFileLines := 100000
	longFileSize := longFileLines * bytesPerLine

	require.NoError(t, featuregate.GlobalRegistry().Set(allowFileDeletion.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(allowFileDeletion.ID(), false))
	}()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.DeleteAfterRead = true
	emitCalls := make(chan *emitParams, longFileLines+1)
	operator := buildTestManagerWithEmit(t, cfg, emitCalls)
	operator.persister = testutil.NewMockPersister("test")

	shortFile := openTemp(t, tempDir)
	_, err := shortFile.WriteString(string(shortFileLine) + "\n")
	require.NoError(t, err)
	require.NoError(t, shortFile.Close())

	longFile := openTemp(t, tempDir)
	for line := 0; line < longFileLines; line++ {
		_, err := longFile.WriteString(string(tokenWithLength(bytesPerLine-1)) + "\n")
		require.NoError(t, err)
	}
	require.NoError(t, longFile.Close())

	// Verify we have no checkpointed files
	require.Equal(t, 0, len(operator.knownFiles))

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

	for !(shortOne && longOne) {
		if line := waitForEmit(t, emitCalls); string(line.token) == string(shortFileLine) {
			shortOne = true
		} else {
			longOne = true
		}
	}

	// Stop consuming before long file has been fully consumed
	cancel()
	wg.Wait()

	// short file was fully consumed and should have been deleted
	require.NoFileExists(t, shortFile.Name())

	// long file was partially consumed and should NOT have been deleted
	require.FileExists(t, longFile.Name())

	// Verify that only long file is remembered and that (0 < offset < fileSize)
	require.Equal(t, 1, len(operator.knownFiles))
	reader := operator.knownFiles[0]
	require.Equal(t, longFile.Name(), reader.file.Name())
	require.Greater(t, reader.Offset, int64(0))
	require.Less(t, reader.Offset, int64(longFileSize))
}

func TestHeaderPersistance(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), false))
	})

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg = cfg.withHeader("^#", "(?P<header_key>[A-z]+): (?P<header_value>[A-z]+)")

	op1, emitCalls1 := buildTestManager(t, cfg)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "#headerField: headerValue\nlog line\n")

	persister := testutil.NewUnscopedMockPersister()
	require.NoError(t, op1.Start(persister))

	waitForTokenHeaderAttributes(t, emitCalls1, []byte("log line"), map[string]any{
		"header_key":   "headerField",
		"header_value": "headerValue",
	})

	require.NoError(t, op1.Stop())

	writeString(t, temp, "log line 2\n")

	op2, emitCalls2 := buildTestManager(t, cfg)

	require.NoError(t, op2.Start(persister))

	waitForTokenHeaderAttributes(t, emitCalls2, []byte("log line 2"), map[string]any{
		"header_key":   "headerField",
		"header_value": "headerValue",
	})

	require.NoError(t, op2.Stop())

}

func TestHeaderPersistanceInHeader(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(AllowHeaderMetadataParsing.ID(), false))
	})

	tempDir := t.TempDir()
	cfg1 := NewConfig().includeDir(tempDir)
	cfg1.StartAt = "beginning"
	cfg1 = cfg1.withHeader(`^\|`, "headerField1: (?P<header_value_1>[A-z0-9]+)")

	op1, _ := buildTestManager(t, cfg1)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "|headerField1: headerValue1\n")

	persister := testutil.NewUnscopedMockPersister()
	require.NoError(t, op1.Start(persister))

	// The operator will poll at fixed time intervals, but we just want to make sure at least
	// one poll operation occurs between now and when we stop.
	op1.poll(context.Background())

	require.NoError(t, op1.Stop())

	writeString(t, temp, "|headerField2: headerValue2\nlog line\n")

	cfg2 := NewConfig().includeDir(tempDir)
	cfg2.StartAt = "beginning"
	cfg2 = cfg2.withHeader(`^\|`, "headerField2: (?P<header_value_2>[A-z0-9]+)")

	op2, emitCalls := buildTestManager(t, cfg2)

	require.NoError(t, op2.Start(persister))

	waitForTokenHeaderAttributes(t, emitCalls, []byte("log line"), map[string]any{
		"header_value_1": "headerValue1",
		"header_value_2": "headerValue2",
	})

	require.NoError(t, op2.Stop())

}

func TestStalePartialFingerprintDiscarded(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.FingerprintSize = 18
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	// Both of they will be include
	file1 := openTempWithPattern(t, tempDir, "*.log1")
	file2 := openTempWithPattern(t, tempDir, "*.log2")

	// Two same fingerprint file , and smaller than  config size
	content := "aaaaaaaaaaa"
	writeString(t, file1, content+"\n")
	writeString(t, file2, content+"\n")
	operator.poll(context.Background())
	// one file will be exclude, ingest only one content
	waitForToken(t, emitCalls, []byte(content))
	expectNoTokens(t, emitCalls)
	operator.wg.Wait()

	// keep append data to file1 and file2
	newContent := "bbbbbbbbbbbb"
	newContent1 := "ddd"
	writeString(t, file1, newContent1+"\n")
	writeString(t, file2, newContent+"\n")
	operator.poll(context.Background())
	// We should have updated the offset for one of the files, so the second file should now
	// be ingested from the beginning
	waitForTokens(t, emitCalls, [][]byte{[]byte(content), []byte(newContent1), []byte(newContent)})
	operator.wg.Wait()
}
