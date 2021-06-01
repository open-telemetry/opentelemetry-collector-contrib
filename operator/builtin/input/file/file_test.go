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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestCleanStop(t *testing.T) {
	t.Parallel()
	t.Skip(`Skipping due to goroutine leak in opencensus.
See this issue for details: https://github.com/census-instrumentation/opencensus-go/issues/1191#issuecomment-610440163`)
	// defer goleak.VerifyNone(t)

	operator, _, tempDir := newTestFileOperator(t, nil, nil)
	_ = openTemp(t, tempDir)
	err := operator.Start(testutil.NewMockPersister("test"))
	require.NoError(t, err)
	operator.Stop()
}

// AddFields tests that the `file_name` and `file_path` fields are included
// when IncludeFileName and IncludeFilePath are set to true
func TestAddFileFields(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *InputConfig) {
		cfg.IncludeFileName = true
		cfg.IncludeFilePath = true
	}, nil)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	e := waitForOne(t, logReceived)
	require.Equal(t, filepath.Base(temp.Name()), e.Attributes["file_name"])
	require.Equal(t, temp.Name(), e.Attributes["file_path"])
}

// ReadExistingLogs tests that, when starting from beginning, we
// read all the lines that are already there
func TestReadExistingLogs(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	// Create a file, then start
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// ReadNewLogs tests that, after starting, if a new file is created
// all the entries in that file are read from the beginning
func TestReadNewLogs(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	operator.persister = testutil.NewMockPersister("test")

	// Poll once so we know this isn't a new file
	operator.poll(context.Background())
	defer operator.Stop()

	// Create a new file
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog\n")

	// Poll a second time after the file has been created
	operator.poll(context.Background())

	// Expect the message to come through
	waitForMessage(t, logReceived, "testlog")
}

// ReadExistingAndNewLogs tests that, on startup, if start_at
// is set to `beginning`, we read the logs that are there, and
// we read any additional logs that are written after startup
func TestReadExistingAndNewLogs(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	operator.persister = testutil.NewMockPersister("test")

	// Start with a file with an entry in it, and expect that entry
	// to come through when we poll for the first time
	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n")
	operator.poll(context.Background())
	waitForMessage(t, logReceived, "testlog1")

	// Write a second entry, and expect that entry to come through
	// as well
	writeString(t, temp, "testlog2\n")
	operator.poll(context.Background())
	waitForMessage(t, logReceived, "testlog2")
}

// StartAtEnd tests that when `start_at` is configured to `end`,
// we don't read any entries that were in the file before startup
func TestStartAtEnd(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, func(cfg *InputConfig) {
		cfg.StartAt = "end"
	}, nil)
	operator.persister = testutil.NewMockPersister("test")

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n")

	// Expect no entries on the first poll
	operator.poll(context.Background())
	expectNoMessages(t, logReceived)

	// Expect any new entries after the first poll
	writeString(t, temp, "testlog2\n")
	operator.poll(context.Background())
	waitForMessage(t, logReceived, "testlog2")
}

// StartAtEndNewFile tests that when `start_at` is configured to `end`,
// a file created after the operator has been started is read from the
// beginning
func TestStartAtEndNewFile(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	operator.persister = testutil.NewMockPersister("test")
	operator.startAtBeginning = false

	operator.poll(context.Background())

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// NoNewline tests that an entry will still be sent eventually
// even if the file doesn't end in a newline
func TestNoNewline(t *testing.T) {
	t.Parallel()
	t.Skip()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\ntestlog2")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// SkipEmpty tests that the any empty lines are skipped
func TestSkipEmpty(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1\n\ntestlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
}

// SplitWrite tests a line written in two writes
// close together still is read as a single entry
func TestSplitWrite(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	operator.persister = testutil.NewMockPersister("test")

	temp := openTemp(t, tempDir)
	writeString(t, temp, "testlog1")

	operator.poll(context.Background())

	writeString(t, temp, "testlog2\n")

	operator.poll(context.Background())
	waitForMessage(t, logReceived, "testlog1testlog2")
}

func TestDecodeBufferIsResized(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	temp := openTemp(t, tempDir)
	expected := stringWithLength(1<<12 + 1)
	writeString(t, temp, expected+"\n")

	waitForMessage(t, logReceived, expected)
}

func TestMultiFileSimple(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	temp1 := openTemp(t, tempDir)
	temp2 := openTemp(t, tempDir)

	writeString(t, temp1, "testlog1\n")
	writeString(t, temp2, "testlog2\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	waitForMessages(t, logReceived, []string{"testlog1", "testlog2"})
}

func TestMultiFileParallel_PreloadedFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

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
	defer operator.Stop()

	waitForMessages(t, logReceived, expected)
	wg.Wait()
}

func TestMultiFileParallel_LiveFiles(t *testing.T) {
	t.Parallel()

	getMessage := func(f, m int) string { return fmt.Sprintf("file %d, message %d", f, m) }

	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	numFiles := 10
	numMessages := 100

	expected := make([]string, 0, numFiles*numMessages)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			expected = append(expected, getMessage(i, j))
		}
	}

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

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
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	persister := testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\n")

	// Start the operator and expect a message
	require.NoError(t, operator.Start(persister))
	defer operator.Stop()
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
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	persister := testutil.NewMockPersister("test")

	log1 := stringWithLength(2000)
	log2 := stringWithLength(2000)

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, log1+"\n")

	// Start the operator
	require.NoError(t, operator.Start(persister))
	defer operator.Stop()
	waitForMessage(t, logReceived, log1)

	// Restart the operator
	require.NoError(t, operator.Stop())
	require.NoError(t, operator.Start(persister))

	writeString(t, temp1, log2+"\n")
	waitForMessage(t, logReceived, log2)
}

func TestOffsetsAfterRestart_BigFilesWrittenWhileOff(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	persister := testutil.NewMockPersister("test")

	log1 := stringWithLength(2000)
	log2 := stringWithLength(2000)

	temp := openTemp(t, tempDir)
	writeString(t, temp, log1+"\n")

	// Start the operator and expect the first message
	require.NoError(t, operator.Start(persister))
	defer operator.Stop()
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
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	count := 1000
	expectedMessages := make([]string, 0, count)
	for i := 0; i < count; i++ {
		expectedMessages = append(expectedMessages, strconv.Itoa(i))
	}

	// Start the operator
	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	// Write lots of logs
	temp := openTemp(t, tempDir)
	for _, message := range expectedMessages {
		temp.WriteString(message + "\n")
	}

	// Expect each of them to come through once
	for _, message := range expectedMessages {
		waitForMessage(t, logReceived, message)
	}
	expectNoMessages(t, logReceived)
}

func TestFileBatching(t *testing.T) {
	t.Parallel()

	files := 100
	linesPerFile := 10
	maxConcurrentFiles := 10

	expectedBatches := files / maxConcurrentFiles // assumes no remainder
	expectedLinesPerBatch := maxConcurrentFiles * linesPerFile

	expectedMessages := make([]string, 0, files*linesPerFile)
	actualMessages := make([]string, 0, files*linesPerFile)

	operator, logReceived, tempDir := newTestFileOperator(t,
		func(cfg *InputConfig) {
			cfg.MaxConcurrentFiles = maxConcurrentFiles
		},
		func(out *testutil.FakeOutput) {
			out.Received = make(chan *entry.Entry, expectedLinesPerBatch)
		},
	)
	operator.persister = testutil.NewMockPersister("test")

	temps := make([]*os.File, 0, files)
	for i := 0; i < files; i++ {
		temps = append(temps, openTemp(t, tempDir))
	}

	// Write logs to each file
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", stringWithLength(100), i, j)
			temp.WriteString(message + "\n")
			expectedMessages = append(expectedMessages, message)
		}
	}

	for b := 0; b < expectedBatches; b++ {
		// poll once so we can validate that files were batched
		operator.poll(context.Background())
		actualMessages = append(actualMessages, waitForN(t, logReceived, expectedLinesPerBatch)...)
		expectNoMessagesUntil(t, logReceived, 10*time.Millisecond)
	}

	require.ElementsMatch(t, expectedMessages, actualMessages)

	// Write more logs to each file so we can validate that all files are still known
	for i, temp := range temps {
		for j := 0; j < linesPerFile; j++ {
			message := fmt.Sprintf("%s %d %d", stringWithLength(20), i, j)
			temp.WriteString(message + "\n")
			expectedMessages = append(expectedMessages, message)
		}
	}

	for b := 0; b < expectedBatches; b++ {
		// poll once so we can validate that files were batched
		operator.poll(context.Background())
		actualMessages = append(actualMessages, waitForN(t, logReceived, expectedLinesPerBatch)...)
		expectNoMessagesUntil(t, logReceived, 10*time.Millisecond)
	}

	require.ElementsMatch(t, expectedMessages, actualMessages)
}

func TestFileReader_FingerprintUpdated(t *testing.T) {
	t.Parallel()

	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	temp := openTemp(t, tempDir)
	tempCopy := openFile(t, temp.Name())
	fp, err := operator.NewFingerprint(temp)
	require.NoError(t, err)
	reader, err := operator.NewReader(temp.Name(), tempCopy, fp)
	require.NoError(t, err)

	writeString(t, temp, "testlog1\n")
	reader.ReadToEnd(context.Background())
	waitForMessage(t, logReceived, "testlog1")
	require.Equal(t, []byte("testlog1\n"), reader.Fingerprint.FirstBytes)
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
			operator, receivedEntries, tempDir := newTestFileOperator(t, func(cfg *InputConfig) {
				cfg.Encoding = helper.EncodingConfig{Encoding: tc.encoding}
			}, nil)

			// Popualte the file
			temp := openTemp(t, tempDir)
			_, err := temp.Write(tc.contents)
			require.NoError(t, err)

			require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
			defer operator.Stop()

			for _, expected := range tc.expected {
				select {
				case entry := <-receivedEntries:
					require.Equal(t, expected, []byte(entry.Body.(string)))
				case <-time.After(500 * time.Millisecond):
					require.FailNow(t, "Timed out waiting for entry to be read")
				}
			}
		})
	}
}

// TestExclude tests that a log file will be excluded if it matches the
// glob specified in the operator.
func TestExclude(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	paths := writeTempFiles(tempDir, []string{"include.log", "exclude.log"})

	includes := []string{filepath.Join(tempDir, "*")}
	excludes := []string{filepath.Join(tempDir, "*exclude.log")}

	matches := getMatches(includes, excludes)
	require.ElementsMatch(t, matches, paths[:1])
}
func TestExcludeEmpty(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	paths := writeTempFiles(tempDir, []string{"include.log", "exclude.log"})

	includes := []string{filepath.Join(tempDir, "*")}
	excludes := []string{}

	matches := getMatches(includes, excludes)
	require.ElementsMatch(t, matches, paths)
}
func TestExcludeMany(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	paths := writeTempFiles(tempDir, []string{"a1.log", "a2.log", "b1.log", "b2.log"})

	includes := []string{filepath.Join(tempDir, "*")}
	excludes := []string{filepath.Join(tempDir, "a*.log"), filepath.Join(tempDir, "*2.log")}

	matches := getMatches(includes, excludes)
	require.ElementsMatch(t, matches, paths[2:3])
}
func TestExcludeDuplicates(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	paths := writeTempFiles(tempDir, []string{"a1.log", "a2.log", "b1.log", "b2.log"})

	includes := []string{filepath.Join(tempDir, "*1*"), filepath.Join(tempDir, "a*")}
	excludes := []string{filepath.Join(tempDir, "a*.log"), filepath.Join(tempDir, "*2.log")}

	matches := getMatches(includes, excludes)
	require.ElementsMatch(t, matches, paths[2:3])
}
