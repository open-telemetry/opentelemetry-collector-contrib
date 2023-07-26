// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

const windowsOS = "windows"

func TestMultiFileRotate(t *testing.T) {
	if runtime.GOOS == windowsOS {
		// Windows has very poor support for moving active files, so rotation is less commonly used
		t.Skip()
	}
	t.Parallel()

	getMessage := func(f, k, m int) string { return fmt.Sprintf("file %d-%d, message %d", f, k, m) }

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	numFiles := 3
	numMessages := 3
	numRotations := 3

	expected := make([][]byte, 0, numFiles*numMessages*numRotations)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			for k := 0; k < numRotations; k++ {
				expected = append(expected, []byte(getMessage(i, k, j)))
			}
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

	waitForTokens(t, emitCalls, expected)
	wg.Wait()
}

func TestMultiFileRotateSlow(t *testing.T) {
	if runtime.GOOS == windowsOS {
		// Windows has very poor support for moving active files, so rotation is less commonly used
		t.Skip()
	}

	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	getMessage := func(f, k, m int) string { return fmt.Sprintf("file %d-%d, message %d", f, k, m) }
	fileName := func(f, k int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.rot%d.log", f, k)) }
	baseFileName := func(f int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.log", f)) }

	numFiles := 3
	numMessages := 30
	numRotations := 3

	expected := make([][]byte, 0, numFiles*numMessages*numRotations)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			for k := 0; k < numRotations; k++ {
				expected = append(expected, []byte(getMessage(i, k, j)))
			}
		}
	}

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

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

	waitForTokens(t, emitCalls, expected)
	wg.Wait()
}

func TestMultiCopyTruncateSlow(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	getMessage := func(f, k, m int) string { return fmt.Sprintf("file %d-%d, message %d", f, k, m) }
	fileName := func(f, k int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.rot%d.log", f, k)) }
	baseFileName := func(f int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.log", f)) }

	numFiles := 3
	numMessages := 30
	numRotations := 3

	expected := make([][]byte, 0, numFiles*numMessages*numRotations)
	for i := 0; i < numFiles; i++ {
		for j := 0; j < numMessages; j++ {
			for k := 0; k < numRotations; k++ {
				expected = append(expected, []byte(getMessage(i, k, j)))
			}
		}
	}

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

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

	waitForTokens(t, emitCalls, expected)
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
 1. Cases that do not require deletion of old log entries should always pass.
 2. Cases where the polling interval is sufficiently rapid should always pass.
 3. When neither 1 nor 2 is true, there may be missing entries, but still no duplicates.

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

		tempDir := t.TempDir()
		cfg := NewConfig().includeDir(tempDir)
		cfg.StartAt = "beginning"
		cfg.PollInterval = tc.pollInterval
		emitCalls := make(chan *emitParams, tc.totalLines)
		operator, _ := buildTestManager(t, cfg, withEmitChan(emitCalls))

		logger := getRotatingLogger(t, tempDir, tc.maxLinesPerFile, tc.maxBackupFiles, copyTruncate, sequential)

		expected := make([][]byte, 0, tc.totalLines)
		baseStr := string(tokenWithLength(46)) // + ' 123'
		for i := 0; i < tc.totalLines; i++ {
			expected = append(expected, []byte(fmt.Sprintf("%s %3d", baseStr, i)))
		}

		require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
		defer func() {
			require.NoError(t, operator.Stop())
		}()

		for _, message := range expected {
			logger.Println(string(message))
			time.Sleep(tc.writeInterval)
		}

		received := make([][]byte, 0, tc.totalLines)
	LOOP:
		for {
			select {
			case call := <-emitCalls:
				received = append(received, call.token)
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
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
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
		if runtime.GOOS != windowsOS {
			// Windows has very poor support for moving active files, so rotation is less commonly used
			t.Run(fmt.Sprintf("%s/MoveCreateTimestamped", tc.name), tc.run(tc, false, false))
			t.Run(fmt.Sprintf("%s/MoveCreateSequential", tc.name), tc.run(tc, false, true))
		}
		t.Run(fmt.Sprintf("%s/CopyTruncateTimestamped", tc.name), tc.run(tc, true, false))
		t.Run(fmt.Sprintf("%s/CopyTruncateSequential", tc.name), tc.run(tc, true, true))
	}
}

func TestMoveFile(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Moving files while open is unsupported on Windows")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\n")
	temp1.Close()

	operator.poll(context.Background())
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))

	// Wait until all goroutines are finished before renaming
	operator.wg.Wait()
	err := os.Rename(temp1.Name(), fmt.Sprintf("%s.2", temp1.Name()))
	require.NoError(t, err)

	operator.poll(context.Background())
	expectNoTokens(t, emitCalls)
}

func TestTrackMovedAwayFiles(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Moving files while open is unsupported on Windows")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\n")
	temp1.Close()

	operator.poll(context.Background())
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))

	// Wait until all goroutines are finished before renaming
	operator.wg.Wait()

	newDir := fmt.Sprintf("%s%s", tempDir[:len(tempDir)-1], "_new/")
	err := os.Mkdir(newDir, 0777)
	require.NoError(t, err)
	newFileName := fmt.Sprintf("%s%s", newDir, "newfile.log")

	err = os.Rename(temp1.Name(), newFileName)
	require.NoError(t, err)

	movedFile, err := os.OpenFile(newFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	writeString(t, movedFile, "testlog2\n")
	operator.poll(context.Background())

	waitForToken(t, emitCalls, []byte("testlog2"))
}

// Check if we read log lines from a rotated file before lines from the newly created file
// Note that we don't guarantee ordering based on file identity - only that we read from rotated files first
func TestTrackRotatedFilesLogOrder(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Moving files while open is unsupported on Windows")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)

	originalFile := openTemp(t, tempDir)
	orginalName := originalFile.Name()
	writeString(t, originalFile, "testlog1\n")

	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))
	writeString(t, originalFile, "testlog2\n")
	originalFile.Close()

	newDir := fmt.Sprintf("%s%s", tempDir[:len(tempDir)-1], "_new/")
	err := os.Mkdir(newDir, 0777)
	require.NoError(t, err)
	movedFileName := fmt.Sprintf("%s%s", newDir, "newfile.log")

	err = os.Rename(orginalName, movedFileName)
	require.NoError(t, err)

	newFile, err := os.OpenFile(orginalName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	writeString(t, newFile, "testlog3\n")

	waitForTokens(t, emitCalls, [][]byte{[]byte("testlog2"), []byte("testlog3")})
}

// TruncateThenWrite tests that, after a file has been truncated,
// any new writes are picked up
func TestTruncateThenWrite(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte("testlog2"))

	require.NoError(t, temp1.Truncate(0))
	_, err := temp1.Seek(0, 0)
	require.NoError(t, err)

	writeString(t, temp1, "testlog3\n")
	operator.poll(context.Background())
	waitForToken(t, emitCalls, []byte("testlog3"))
	expectNoTokens(t, emitCalls)
}

// CopyTruncateWriteBoth tests that when a file is copied
// with unread logs on the end, then the original is truncated,
// we get the unread logs on the copy as well as any new logs
// written to the truncated file
func TestCopyTruncateWriteBoth(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")

	temp1 := openTemp(t, tempDir)
	writeString(t, temp1, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	waitForToken(t, emitCalls, []byte("testlog1"))
	waitForToken(t, emitCalls, []byte("testlog2"))
	operator.wg.Wait() // wait for all goroutines to finish

	// Copy the first file to a new file, and add another log
	temp2 := openTemp(t, tempDir)
	_, err := io.Copy(temp2, temp1)
	require.NoError(t, err)

	// Truncate original file
	require.NoError(t, temp1.Truncate(0))
	_, err = temp1.Seek(0, 0)
	require.NoError(t, err)

	// Write to original and new file
	writeString(t, temp2, "testlog3\n")
	writeString(t, temp1, "testlog4\n")

	// Expect both messages to come through
	operator.poll(context.Background())
	waitForTokens(t, emitCalls, [][]byte{[]byte("testlog3"), []byte("testlog4")})
}

func TestFileMovedWhileOff_BigFiles(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	persister := testutil.NewMockPersister("test")

	log1 := tokenWithLength(1000)
	log2 := tokenWithLength(1000)

	temp := openTemp(t, tempDir)
	writeString(t, temp, string(log1)+"\n")
	require.NoError(t, temp.Close())

	// Start the operator
	require.NoError(t, operator.Start(persister))
	defer func() {
		require.NoError(t, operator.Stop())
	}()
	waitForToken(t, emitCalls, log1)

	// Stop the operator, then rename and write a new log
	require.NoError(t, operator.Stop())

	err := os.Rename(temp.Name(), fmt.Sprintf("%s2", temp.Name()))
	require.NoError(t, err)

	temp = reopenTemp(t, temp.Name())
	require.NoError(t, err)
	writeString(t, temp, string(log2)+"\n")

	// Expect the message written to the new log to come through
	require.NoError(t, operator.Start(persister))
	waitForToken(t, emitCalls, log2)
}
