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

package file

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestCleanStop(t *testing.T) {
	t.Parallel()
	t.Skip(`Skipping due to goroutine leak in opencensus.
See this issue for details: https://github.com/census-instrumentation/opencensus-go/issues/1191#issuecomment-610440163`)
	// defer goleak.VerifyNone(t)

	operator, _, tempDir := newTestFileOperator(t, nil)
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
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
		cfg.IncludeFileName = true
		cfg.IncludeFilePath = true
	})

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	e := waitForOne(t, logReceived)
	require.Equal(t, filepath.Base(temp.Name()), e.Attributes["log.file.name"])
	require.Equal(t, temp.Name(), e.Attributes["log.file.path"])
}

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
	real, err := filepath.EvalSymlinks(file.Name())
	require.NoError(t, err)
	resolved, err := filepath.Abs(real)
	require.NoError(t, err)

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	e := waitForOne(t, logReceived)
	require.Equal(t, filepath.Base(symLinkPath), e.Attributes["log.file.name"])
	require.Equal(t, symLinkPath, e.Attributes["log.file.path"])
	require.Equal(t, filepath.Base(resolved), e.Attributes["log.file.name_resolved"])
	require.Equal(t, resolved, e.Attributes["log.file.path_resolved"])
}

// AddFileResolvedFields tests that the `log.file.name_resolved` and `log.file.path_resolved` fields are included
// when IncludeFileNameResolved and IncludeFilePathResolved are set to true and underlaying symlink change
// Scenario:
// monitored file (symlink) -> middleSymlink -> file_1
// monitored file (symlink) -> middleSymlink -> file_2
func TestAddFileResolvedFieldsWithChangeOfSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlinks usage disabled for now. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21088")
	}
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
		cfg.IncludeFileName = true
		cfg.IncludeFilePath = true
		cfg.IncludeFileNameResolved = true
		cfg.IncludeFilePathResolved = true
	})

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

	e := waitForOne(t, logReceived)
	require.Equal(t, filepath.Base(symLinkPath), e.Attributes["log.file.name"])
	require.Equal(t, symLinkPath, e.Attributes["log.file.path"])
	require.Equal(t, filepath.Base(resolved1), e.Attributes["log.file.name_resolved"])
	require.Equal(t, resolved1, e.Attributes["log.file.path_resolved"])

	// Change middleSymLink to point to file2
	err = os.Remove(middleSymLinkPath)
	require.NoError(t, err)
	err = os.Symlink(file2.Name(), middleSymLinkPath)
	require.NoError(t, err)

	// Populate data (different content due to fingerprint)
	writeString(t, file2, "testlog2\n")

	e = waitForOne(t, logReceived)
	require.Equal(t, filepath.Base(symLinkPath), e.Attributes["log.file.name"])
	require.Equal(t, symLinkPath, e.Attributes["log.file.path"])
	require.Equal(t, filepath.Base(resolved2), e.Attributes["log.file.name_resolved"])
	require.Equal(t, resolved2, e.Attributes["log.file.path_resolved"])
}

// ReadExistingLogs tests that, when starting from beginning, we
// read all the lines that are already there
func TestReadExistingLogs(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
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
				cfg.Splitter.EncodingConfig.Encoding = "nop"
			})
			// Create a file, then start
			temp := openTemp(t, tempDir)
			bytesWritten, err := temp.Write(tc.input)
			require.Greater(t, bytesWritten, 0)
			require.NoError(t, err)
			require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
			defer func() {
				require.NoError(t, operator.Stop())
			}()

			tc.test(t, logReceived)
		})
	}
}

func TestNopEncodingDifferentLogSizes(t *testing.T) {
	tcs := []struct {
		testName   string
		input      []byte
		test       func(*testing.T, chan *entry.Entry)
		maxLogSize helper.ByteSize
	}{
		{
			"same size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog1"))
			},
			8,
		},
		{
			"massive log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog1"))
			},
			8000000,
		},
		{
			"slightly larger log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog1"))
			},
			9,
		},
		{
			"slightly smaller log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("testlog"))
				waitForByteMessage(t, c, []byte("1"))
			},
			7,
		},
		{
			"tiny log size",
			[]byte("testlog1"),
			func(t *testing.T, c chan *entry.Entry) {
				waitForByteMessage(t, c, []byte("t"))
				waitForByteMessage(t, c, []byte("e"))
				waitForByteMessage(t, c, []byte("s"))
				waitForByteMessage(t, c, []byte("t"))
				waitForByteMessage(t, c, []byte("l"))
				waitForByteMessage(t, c, []byte("o"))
				waitForByteMessage(t, c, []byte("g"))
				waitForByteMessage(t, c, []byte("1"))
			},
			1,
		},
	}

	t.Parallel()

	for _, tc := range tcs {
		t.Run(tc.testName, func(t *testing.T) {
			operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
				cfg.MaxLogSize = tc.maxLogSize
				cfg.Splitter.EncodingConfig.Encoding = "nop"
			})
			// Create a file, then start
			temp := openTemp(t, tempDir)
			bytesWritten, err := temp.Write(tc.input)
			require.Greater(t, bytesWritten, 0)
			require.NoError(t, err)
			require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
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

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
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

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
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

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	time.Sleep(2 * pollInterval)

	expectNoMessages(t, logReceived)

	// Expect any new entries after the first poll
	writeString(t, temp, "testlog2\n")
	waitForMessage(t, logReceived, "testlog2")
}

// StartAtEndNewFile tests that when `start_at` is configured to `end`,
// a file created after the operator has been started is read from the
// beginning
func TestStartAtEndNewFile(t *testing.T) {
	t.Parallel()

	var pollInterval time.Duration
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
		cfg.StartAt = "end"
		pollInterval = cfg.PollInterval
	})

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	time.Sleep(2 * pollInterval)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\n")

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// NoNewline tests that an entry will still be sent eventually
// even if the file doesn't end in a newline
func TestNoNewline(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *Config) {
		cfg.Splitter = helper.NewSplitterConfig()
		cfg.Splitter.Flusher.Period = time.Nanosecond
	})

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// SkipEmpty tests that the any empty lines are skipped
func TestSkipEmpty(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// SplitWrite tests a line written in two writes
// close together still is read as a single entry
func TestSplitWrite(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1")
	writeString(t, temp, "testlog2\n")

	waitForMessage(t, logReceived, "testlog1testlog2")
}

func TestIgnoreEmptyFiles(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	temp := openTemp(t, tempDir)
	temp2 := openTemp(t, tempDir)
	temp3 := openTemp(t, tempDir)
	temp4 := openTemp(t, tempDir)

	writeString(t, temp, "testlog1\n")
	writeString(t, temp3, "testlog2\n")
	waitForMessages(t, logReceived, []string{"testlog1", "testlog2"})

	writeString(t, temp2, "testlog3\n")
	writeString(t, temp4, "testlog4\n")
	waitForMessages(t, logReceived, []string{"testlog3", "testlog4"})
}

func TestDecodeBufferIsResized(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	temp := openTemp(t, tempDir)
	expected := stringWithLength(1<<12 + 1)
	writeString(t, temp, expected+"\n")

	waitForMessage(t, logReceived, expected)
}

func TestMultiFileSimple(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	temp1 := openTemp(t, tempDir)
	temp2 := openTemp(t, tempDir)

	writeString(t, temp1, "testlog1\n")
	writeString(t, temp2, "testlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForMessages(t, logReceived, []string{"testlog1", "testlog2"})
}

func TestMultiFileParallel_PreloadedFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	numFiles := 10
	numMessages := 100

	expected := make([]string, 0, numFiles*numMessages)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			expected = append(expected, getMessage(i, j))
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

	waitForMessages(t, logReceived, expected)
	wg.Wait()
}

func TestMultiFileParallel_LiveFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	numFiles := 10
	numMessages := 100

	expected := make([]string, 0, numFiles*numMessages)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			expected = append(expected, getMessage(i, j))
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

	waitForMessages(t, logReceived, expected)
	wg.Wait()
}

// OffsetsAfterRestart tests that a operator is able to load
// its offsets after a restart
func TestOffsetsAfterRestart(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)
	persister := testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\n")

	// Start the operator and expect a message
	require.NoError(t, operator.Start(persister))
	defer func() {
		require.NoError(t, operator.Stop())
	}()
	waitForMessage(t, logReceived, "testlog1")

	// Restart the operator. Stop and build a new
	// one to guarantee freshness
	require.NoError(t, operator.Stop())
	require.NoError(t, operator.Start(persister))

	// Write a new log and expect only that log
	writeString(t, temp1, "testlog2\n")
	waitForMessage(t, logReceived, "testlog2")
}

func TestOffsetsAfterRestart_BigFiles(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)
	persister := testutil.NewMockPersister("test")

	log1 := stringWithLength(2000)
	log2 := stringWithLength(2000)

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, log1+"\n")

	// Start the operator
	require.NoError(t, operator.Start(persister))
	defer func() {
		require.NoError(t, operator.Stop())
	}()
	waitForMessage(t, logReceived, log1)

	// Restart the operator
	require.NoError(t, operator.Stop())
	require.NoError(t, operator.Start(persister))

	writeString(t, temp1, log2+"\n")
	waitForMessage(t, logReceived, log2)
}

func TestOffsetsAfterRestart_BigFilesWrittenWhileOff(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)
	persister := testutil.NewMockPersister("test")

	log1 := stringWithLength(2000)
	log2 := stringWithLength(2000)

	temp := openTemp(t, tempDir)
	writeString(t, temp, log1+"\n")

	// Start the operator and expect the first message
	require.NoError(t, operator.Start(persister))
	defer func() {
		require.NoError(t, operator.Stop())
	}()
	waitForMessage(t, logReceived, log1)

	// Stop the operator and write a new message
	require.NoError(t, operator.Stop())
	writeString(t, temp, log2+"\n")

	// Start the operator and expect the message
	require.NoError(t, operator.Start(persister))
	waitForMessage(t, logReceived, log2)
}

func TestManyLogsDelivered(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil)

	count := 1000
	expectedMessages := make([]string, 0, count)
	for i := 0; i < count; i++ {
		expectedMessages = append(expectedMessages, strconv.Itoa(i))
	}

	// Start the operator
	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	// Write lots of logs
	temp := openTemp(t, tempDir)
	for _, message := range expectedMessages {
		_, err := temp.WriteString(message + "\n")
		require.NoError(t, err)
	}

	// Expect each of them to come through once
	for _, message := range expectedMessages {
		waitForMessage(t, logReceived, message)
	}
	expectNoMessages(t, logReceived)
}
