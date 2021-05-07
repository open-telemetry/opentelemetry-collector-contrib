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
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/observiq/nanojack"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func newDefaultConfig(tempDir string) *InputConfig {
	cfg := NewInputConfig("testfile")
	cfg.PollInterval = helper.Duration{Duration: 50 * time.Millisecond}
	cfg.StartAt = "beginning"
	cfg.Include = []string{fmt.Sprintf("%s/*", tempDir)}
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func newTestFileOperator(t *testing.T, cfgMod func(*InputConfig), outMod func(*testutil.FakeOutput)) (*InputOperator, chan *entry.Entry, string) {
	fakeOutput := testutil.NewFakeOutput(t)
	if outMod != nil {
		outMod(fakeOutput)
	}

	tempDir := testutil.NewTempDir(t)

	cfg := newDefaultConfig(tempDir)
	if cfgMod != nil {
		cfgMod(cfg)
	}
	ops, err := cfg.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
	op := ops[0]

	err = op.SetOutputs([]operator.Operator{fakeOutput})
	require.NoError(t, err)

	return op.(*InputOperator), fakeOutput.Received, tempDir
}

func openFile(t testing.TB, path string) *os.File {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0777)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func openTemp(t testing.TB, tempDir string) *os.File {
	return openTempWithPattern(t, tempDir, "")
}

func reopenTemp(t testing.TB, name string) *os.File {
	return openTempWithPattern(t, filepath.Dir(name), filepath.Base(name))
}

func openTempWithPattern(t testing.TB, tempDir, pattern string) *os.File {
	file, err := ioutil.TempFile(tempDir, pattern)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func getRotatingLogger(t testing.TB, tempDir string, maxLines, maxBackups int, copyTruncate, sequential bool) *log.Logger {
	file, err := ioutil.TempFile(tempDir, "")
	require.NoError(t, err)
	require.NoError(t, file.Close()) // will be managed by rotator

	rotator := nanojack.Logger{
		Filename:     file.Name(),
		MaxLines:     maxLines,
		MaxBackups:   maxBackups,
		CopyTruncate: copyTruncate,
		Sequential:   sequential,
	}

	t.Cleanup(func() { _ = rotator.Close() })

	return log.New(&rotator, "", 0)
}

func writeString(t testing.TB, file *os.File, s string) {
	_, err := file.WriteString(s)
	require.NoError(t, err)
}

func TestBuild(t *testing.T) {
	t.Parallel()
	fakeOutput := testutil.NewMockOperator("$.fake")

	basicConfig := func() *InputConfig {
		cfg := NewInputConfig("testfile")
		cfg.OutputIDs = []string{"fake"}
		cfg.Include = []string{"/var/log/testpath.*"}
		cfg.Exclude = []string{"/var/log/testpath.ex*"}
		cfg.PollInterval = helper.Duration{Duration: 10 * time.Millisecond}
		return cfg
	}

	cases := []struct {
		name             string
		modifyBaseConfig func(*InputConfig)
		errorRequirement require.ErrorAssertionFunc
		validate         func(*testing.T, *InputOperator)
	}{
		{
			"Basic",
			func(f *InputConfig) {},
			require.NoError,
			func(t *testing.T, f *InputOperator) {
				require.Equal(t, f.OutputOperators[0], fakeOutput)
				require.Equal(t, f.Include, []string{"/var/log/testpath.*"})
				require.Equal(t, f.FilePathField, entry.NewNilField())
				require.Equal(t, f.FileNameField, entry.NewAttributeField("file_name"))
				require.Equal(t, f.PollInterval, 10*time.Millisecond)
			},
		},
		{
			"BadIncludeGlob",
			func(f *InputConfig) {
				f.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"BadExcludeGlob",
			func(f *InputConfig) {
				f.Include = []string{"["}
			},
			require.Error,
			nil,
		},
		{
			"MultilineConfiguredStartAndEndPatterns",
			func(f *InputConfig) {
				f.Multiline = helper.MultilineConfig{
					LineEndPattern:   "Exists",
					LineStartPattern: "Exists",
				}
			},
			require.Error,
			nil,
		},
		{
			"MultilineConfiguredStartPattern",
			func(f *InputConfig) {
				f.Multiline = helper.MultilineConfig{
					LineStartPattern: "START.*",
				}
			},
			require.NoError,
			func(t *testing.T, f *InputOperator) {},
		},
		{
			"MultilineConfiguredEndPattern",
			func(f *InputConfig) {
				f.Multiline = helper.MultilineConfig{
					LineEndPattern: "END.*",
				}
			},
			require.NoError,
			func(t *testing.T, f *InputOperator) {},
		},
		{
			"InvalidEncoding",
			func(f *InputConfig) {
				f.Encoding = helper.EncodingConfig{Encoding: "UTF-3233"}
			},
			require.Error,
			nil,
		},
		{
			"LineStartAndEnd",
			func(f *InputConfig) {
				f.Multiline = helper.MultilineConfig{
					LineStartPattern: ".*",
					LineEndPattern:   ".*",
				}
			},
			require.Error,
			nil,
		},
		{
			"NoLineStartOrEnd",
			func(f *InputConfig) {
				f.Multiline = helper.MultilineConfig{}
			},
			require.NoError,
			func(t *testing.T, f *InputOperator) {},
		},
		{
			"InvalidLineStartRegex",
			func(f *InputConfig) {
				f.Multiline = helper.MultilineConfig{
					LineStartPattern: "(",
				}
			},
			require.Error,
			nil,
		},
		{
			"InvalidLineEndRegex",
			func(f *InputConfig) {
				f.Multiline = helper.MultilineConfig{
					LineEndPattern: "(",
				}
			},
			require.Error,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			t.Parallel()
			cfg := basicConfig()
			tc.modifyBaseConfig(cfg)

			ops, err := cfg.Build(testutil.NewBuildContext(t))
			tc.errorRequirement(t, err)
			if err != nil {
				return
			}
			op := ops[0]

			err = op.SetOutputs([]operator.Operator{fakeOutput})
			require.NoError(t, err)

			fileInput := op.(*InputOperator)
			tc.validate(t, fileInput)
		})
	}
}

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

func TestMultiFileRotate(t *testing.T) {
	t.Parallel()

	getMessage := func(f, k, m int) string { return fmt.Sprintf("file %d-%d, message %d", f, k, m) }

	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	numFiles := 3
	numMessages := 3
	numRotations := 3

	expected := make([]string, 0, numFiles*numMessages*numRotations)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			for k := 0; k < numRotations; k++ {
				expected = append(expected, getMessage(i, k, j))
			}
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
			for k := 0; k < numRotations; k++ {
				for j := 0; j < numMessages; j++ {
					writeString(t, tf, getMessage(f, k, j)+"\n")
				}

				require.NoError(t, tf.Close())
				require.NoError(t, os.Rename(tf.Name(), fmt.Sprintf("%s.%d", tf.Name(), k)))
				tf = reopenTemp(t, tf.Name())
			}
		}(temp, i)
	}

	waitForMessages(t, logReceived, expected)
	wg.Wait()
}

func TestMultiFileRotateSlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows has very poor support for moving active files, so rotation is less commonly used
		// This may possibly be handled better in Go 1.16: https://github.com/golang/go/issues/35358
		t.Skip()
	}

	t.Parallel()

	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	getMessage := func(f, k, m int) string { return fmt.Sprintf("file %d-%d, message %d", f, k, m) }
	fileName := func(f, k int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.rot%d.log", f, k)) }
	baseFileName := func(f int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.log", f)) }

	numFiles := 3
	numMessages := 30
	numRotations := 3

	expected := make([]string, 0, numFiles*numMessages*numRotations)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			for k := 0; k < numRotations; k++ {
				expected = append(expected, getMessage(i, k, j))
			}
		}
	}

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	var wg sync.WaitGroup
	for fileNum := 0; fileNum < numFiles; fileNum++ {
		wg.Add(1)
		go func(fn int) {
			defer wg.Done()

			for rotationNum := 0; rotationNum < numRotations; rotationNum++ {
				file := openFile(t, baseFileName(fn))
				for messageNum := 0; messageNum < numMessages; messageNum++ {
					writeString(t, file, getMessage(fn, rotationNum, messageNum)+"\n")
					time.Sleep(5 * time.Millisecond)
				}

				require.NoError(t, file.Close())
				require.NoError(t, os.Rename(baseFileName(fn), fileName(fn, rotationNum)))
			}
		}(fileNum)
	}

	waitForMessages(t, logReceived, expected)
	wg.Wait()
}

func TestMultiCopyTruncateSlow(t *testing.T) {
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)

	getMessage := func(f, k, m int) string { return fmt.Sprintf("file %d-%d, message %d", f, k, m) }
	fileName := func(f, k int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.rot%d.log", f, k)) }
	baseFileName := func(f int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.log", f)) }

	numFiles := 3
	numMessages := 30
	numRotations := 3

	expected := make([]string, 0, numFiles*numMessages*numRotations)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			for k := 0; k < numRotations; k++ {
				expected = append(expected, getMessage(i, k, j))
			}
		}
	}

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer operator.Stop()

	var wg sync.WaitGroup
	for fileNum := 0; fileNum < numFiles; fileNum++ {
		wg.Add(1)
		go func(fn int) {
			defer wg.Done()

			for rotationNum := 0; rotationNum < numRotations; rotationNum++ {
				file := openFile(t, baseFileName(fn))
				for messageNum := 0; messageNum < numMessages; messageNum++ {
					writeString(t, file, getMessage(fn, rotationNum, messageNum)+"\n")
					time.Sleep(5 * time.Millisecond)
				}

				_, err := file.Seek(0, 0)
				require.NoError(t, err)
				dst := openFile(t, fileName(fn, rotationNum))
				_, err = io.Copy(dst, file)
				require.NoError(t, err)
				require.NoError(t, dst.Close())
				require.NoError(t, file.Truncate(0))
				_, err = file.Seek(0, 0)
				require.NoError(t, err)
				file.Close()
			}
		}(fileNum)
	}

	waitForMessages(t, logReceived, expected)
	wg.Wait()
}

type rotationTest struct {
	name            string
	totalLines      int
	maxLinesPerFile int
	maxBackupFiles  int
	writeInterval   time.Duration
	pollInterval    time.Duration
	ephemeralLines  bool
}

/*
	When log files are rotated at extreme speeds, it is possible to miss some log entries.
	This can happen when an individual log entry is written and deleted within the duration
	of a single poll interval. For example, consider the following scenario:
		- A log file may have up to 9 backups (10 total log files)
		- Each log file may contain up to 10 entries
		- Log entries are written at an interval of 10µs
		- Log files are polled at an interval of 100ms
	In this scenario, a log entry that is written may only exist on disk for about 1ms.
	A polling interval of 100ms will most likely never produce a chance to read the log file.

	In production settings, this consideration is not very likely to be a problem, but it is
	easy to encounter the issue in tests, and difficult to deterministically simulate edge cases.
	However, the above understanding does allow for some consistent expectations.
		1) Cases that do not require deletion of old log entries should always pass.
		2) Cases where the polling interval is sufficiently rapid should always pass.
		3) When neither 1 nor 2 is true, there may be missing entries, but still no duplicates.

	The following method is provided largely as documentation of how this is expected to behave.
	In practice, timing is largely dependent on the responsiveness of system calls.
*/
func (rt rotationTest) expectEphemeralLines() bool {
	// primary + backups
	maxLinesInAllFiles := rt.maxLinesPerFile + rt.maxLinesPerFile*rt.maxBackupFiles

	// Will the test write enough lines to result in deletion of oldest backups?
	maxBackupsExceeded := rt.totalLines > maxLinesInAllFiles

	// last line written in primary file will exist for l*b more writes
	minTimeToLive := time.Duration(int(rt.writeInterval) * rt.maxLinesPerFile * rt.maxBackupFiles)

	// can a line be written and then rotated to deletion before ever observed?
	return maxBackupsExceeded && rt.pollInterval > minTimeToLive
}

func (rt rotationTest) run(tc rotationTest, copyTruncate, sequential bool) func(t *testing.T) {
	return func(t *testing.T) {
		operator, logReceived, tempDir := newTestFileOperator(t,
			func(cfg *InputConfig) {
				cfg.PollInterval = helper.NewDuration(tc.pollInterval)
			},
			func(out *testutil.FakeOutput) {
				out.Received = make(chan *entry.Entry, tc.totalLines)
			},
		)
		logger := getRotatingLogger(t, tempDir, tc.maxLinesPerFile, tc.maxBackupFiles, copyTruncate, sequential)

		expected := make([]string, 0, tc.totalLines)
		baseStr := stringWithLength(46) // + ' 123'
		for i := 0; i < tc.totalLines; i++ {
			expected = append(expected, fmt.Sprintf("%s %3d", baseStr, i))
		}

		require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
		defer operator.Stop()

		for _, message := range expected {
			logger.Println(message)
			time.Sleep(tc.writeInterval)
		}

		received := make([]string, 0, tc.totalLines)
	LOOP:
		for {
			select {
			case e := <-logReceived:
				received = append(received, e.Body.(string))
			case <-time.After(200 * time.Millisecond):
				break LOOP
			}
		}

		if tc.ephemeralLines {
			if !tc.expectEphemeralLines() {
				// This is helpful for test development, and ensures the sample computation is used
				t.Logf("Potentially unstable ephemerality expectation for test: %s", tc.name)
			}
			require.Subset(t, expected, received)
		} else {
			require.ElementsMatch(t, expected, received)
		}
	}
}

func TestRotation(t *testing.T) {
	cases := []rotationTest{
		{
			name:            "NoRotation",
			totalLines:      10,
			maxLinesPerFile: 10,
			maxBackupFiles:  1,
			writeInterval:   time.Millisecond,
			pollInterval:    10 * time.Millisecond,
		},
		{
			name:            "NoDeletion",
			totalLines:      20,
			maxLinesPerFile: 10,
			maxBackupFiles:  1,
			writeInterval:   time.Millisecond,
			pollInterval:    10 * time.Millisecond,
		},
		{
			name:            "Deletion",
			totalLines:      30,
			maxLinesPerFile: 10,
			maxBackupFiles:  1,
			writeInterval:   time.Millisecond,
			pollInterval:    10 * time.Millisecond,
			ephemeralLines:  true,
		},
		{
			name:            "Deletion/ExceedFingerprint",
			totalLines:      300,
			maxLinesPerFile: 100,
			maxBackupFiles:  1,
			writeInterval:   time.Millisecond,
			pollInterval:    10 * time.Millisecond,
			ephemeralLines:  true,
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/MoveCreateTimestamped", tc.name), tc.run(tc, false, false))
		t.Run(fmt.Sprintf("%s/MoveCreateSequential", tc.name), tc.run(tc, false, true))
		t.Run(fmt.Sprintf("%s/CopyTruncateTimestamped", tc.name), tc.run(tc, true, false))
		t.Run(fmt.Sprintf("%s/CopyTruncateSequential", tc.name), tc.run(tc, true, true))
	}
}

func TestMoveFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Moving files while open is unsupported on Windows")
	}
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	operator.persister = testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\n")
	temp1.Close()

	operator.poll(context.Background())
	defer operator.Stop()

	waitForMessage(t, logReceived, "testlog1")

	// Wait until all goroutines are finished before renaming
	operator.wg.Wait()
	err := os.Rename(temp1.Name(), fmt.Sprintf("%s.2", temp1.Name()))
	require.NoError(t, err)

	operator.poll(context.Background())
	expectNoMessages(t, logReceived)
}

// TruncateThenWrite tests that, after a file has been truncated,
// any new writes are picked up
func TestTruncateThenWrite(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	operator.persister = testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	defer operator.Stop()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")

	require.NoError(t, temp1.Truncate(0))
	temp1.Seek(0, 0)

	writeString(t, temp1, "testlog3\n")
	operator.poll(context.Background())
	waitForMessage(t, logReceived, "testlog3")
	expectNoMessages(t, logReceived)
}

// CopyTruncateWriteBoth tests that when a file is copied
// with unread logs on the end, then the original is truncated,
// we get the unread logs on the copy as well as any new logs
// written to the truncated file
func TestCopyTruncateWriteBoth(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	operator.persister = testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	defer operator.Stop()

	waitForMessage(t, logReceived, "testlog1")
	waitForMessage(t, logReceived, "testlog2")
	operator.wg.Wait() // wait for all goroutines to finish

	// Copy the first file to a new file, and add another log
	temp2 := openTemp(t, tempDir)
	_, err := io.Copy(temp2, temp1)
	require.NoError(t, err)

	// Truncate original file
	require.NoError(t, temp1.Truncate(0))
	temp1.Seek(0, 0)

	// Write to original and new file
	writeString(t, temp2, "testlog3\n")
	writeString(t, temp1, "testlog4\n")

	// Expect both messages to come through
	operator.poll(context.Background())
	waitForMessages(t, logReceived, []string{"testlog3", "testlog4"})
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

func TestFileMovedWhileOff_BigFiles(t *testing.T) {
	t.Parallel()
	operator, logReceived, tempDir := newTestFileOperator(t, nil, nil)
	persister := testutil.NewMockPersister("test")

	log1 := stringWithLength(1000)
	log2 := stringWithLength(1000)

	temp := openTemp(t, tempDir)
	writeString(t, temp, log1+"\n")
	require.NoError(t, temp.Close())

	// Start the operator
	require.NoError(t, operator.Start(persister))
	defer operator.Stop()
	waitForMessage(t, logReceived, log1)

	// Stop the operator, then rename and write a new log
	require.NoError(t, operator.Stop())

	err := os.Rename(temp.Name(), fmt.Sprintf("%s2", temp.Name()))
	require.NoError(t, err)

	temp = reopenTemp(t, temp.Name())
	require.NoError(t, err)
	writeString(t, temp, log2+"\n")

	// Expect the message written to the new log to come through
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

func stringWithLength(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func waitForOne(t *testing.T, c chan *entry.Entry) *entry.Entry {
	select {
	case e := <-c:
		return e
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for message")
		return nil
	}
}

func waitForN(t *testing.T, c chan *entry.Entry, n int) []string {
	messages := make([]string, 0, n)
	for i := 0; i < n; i++ {
		select {
		case e := <-c:
			messages = append(messages, e.Body.(string))
		case <-time.After(time.Second):
			require.FailNow(t, "Timed out waiting for message")
			return nil
		}
	}
	return messages
}

func waitForMessage(t *testing.T, c chan *entry.Entry, expected string) {
	select {
	case e := <-c:
		require.Equal(t, expected, e.Body.(string))
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for message", expected)
	}
}

func waitForMessages(t *testing.T, c chan *entry.Entry, expected []string) {
	receivedMessages := make([]string, 0, len(expected))
LOOP:
	for {
		select {
		case e := <-c:
			receivedMessages = append(receivedMessages, e.Body.(string))
		case <-time.After(time.Second):
			break LOOP
		}
	}

	require.ElementsMatch(t, expected, receivedMessages)
}

func expectNoMessages(t *testing.T, c chan *entry.Entry) {
	expectNoMessagesUntil(t, c, 200*time.Millisecond)
}

func expectNoMessagesUntil(t *testing.T, c chan *entry.Entry, d time.Duration) {
	select {
	case e := <-c:
		require.FailNow(t, "Received unexpected message", "Message: %s", e.Body.(string))
	case <-time.After(d):
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

type fileInputBenchmark struct {
	name   string
	config *InputConfig
}

func BenchmarkFileInput(b *testing.B) {
	cases := []fileInputBenchmark{
		{
			"Default",
			NewInputConfig("test_id"),
		},
		{
			"NoFileName",
			func() *InputConfig {
				cfg := NewInputConfig("test_id")
				cfg.IncludeFileName = false
				return cfg
			}(),
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			tempDir := testutil.NewTempDir(b)
			path := filepath.Join(tempDir, "in.log")

			cfg := tc.config
			cfg.OutputIDs = []string{"fake"}
			cfg.Include = []string{path}
			cfg.StartAt = "beginning"

			ops, err := cfg.Build(testutil.NewBuildContext(b))
			require.NoError(b, err)
			op := ops[0]

			fakeOutput := testutil.NewFakeOutput(b)
			err = op.SetOutputs([]operator.Operator{fakeOutput})
			require.NoError(b, err)

			err = op.Start(testutil.NewMockPersister("test"))
			defer op.Stop()
			require.NoError(b, err)

			file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
			require.NoError(b, err)

			for i := 0; i < b.N; i++ {
				file.WriteString("testlog\n")
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				<-fakeOutput.Received
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

// writes file with the specified file names and returns their full paths in order
func writeTempFiles(tempDir string, names []string) []string {
	result := make([]string, 0, len(names))
	for _, name := range names {
		path := filepath.Join(tempDir, name)
		ioutil.WriteFile(path, []byte(name), 0755)
		result = append(result, path)
	}
	return result
}
