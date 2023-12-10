// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/observiq/nanojack"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
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
	operator, sink := testManager(t, cfg)

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
			for k := 0; k < numRotations; k++ {
				for j := 0; j < numMessages; j++ {
					filetest.WriteString(t, tf, getMessage(f, k, j)+"\n")
				}

				require.NoError(t, tf.Close())
				require.NoError(t, os.Rename(tf.Name(), fmt.Sprintf("%s.%d", tf.Name(), k)))
				tf = filetest.ReopenTemp(t, tf.Name())
			}
		}(temp, i)
	}

	sink.ExpectTokens(t, expected...)
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
	operator, sink := testManager(t, cfg)

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

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	var wg sync.WaitGroup
	for fileNum := 0; fileNum < numFiles; fileNum++ {
		wg.Add(1)
		go func(fn int) {
			defer wg.Done()

			for rotationNum := 0; rotationNum < numRotations; rotationNum++ {
				file := filetest.OpenFile(t, baseFileName(fn))
				for messageNum := 0; messageNum < numMessages; messageNum++ {
					filetest.WriteString(t, file, getMessage(fn, rotationNum, messageNum)+"\n")
					time.Sleep(5 * time.Millisecond)
				}

				require.NoError(t, file.Close())
				require.NoError(t, os.Rename(baseFileName(fn), fileName(fn, rotationNum)))
			}
		}(fileNum)
	}

	sink.ExpectTokens(t, expected...)
	wg.Wait()
}

func TestMultiCopyTruncateSlow(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)

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

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	var wg sync.WaitGroup
	for fileNum := 0; fileNum < numFiles; fileNum++ {
		wg.Add(1)
		go func(fn int) {
			defer wg.Done()

			for rotationNum := 0; rotationNum < numRotations; rotationNum++ {
				file := filetest.OpenFile(t, baseFileName(fn))
				for messageNum := 0; messageNum < numMessages; messageNum++ {
					filetest.WriteString(t, file, getMessage(fn, rotationNum, messageNum)+"\n")
					time.Sleep(5 * time.Millisecond)
				}

				_, err := file.Seek(0, 0)
				require.NoError(t, err)
				dst := filetest.OpenFile(t, fileName(fn, rotationNum))
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

	sink.ExpectTokens(t, expected...)
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
		sink := emittest.NewSink(emittest.WithCallBuffer(tc.totalLines))
		operator := testManagerWithSink(t, cfg, sink)

		file, err := os.CreateTemp(tempDir, "")
		require.NoError(t, err)
		require.NoError(t, file.Close()) // will be managed by rotator

		rotator := nanojack.Logger{
			Filename:     file.Name(),
			MaxLines:     tc.maxLinesPerFile,
			MaxBackups:   tc.maxBackupFiles,
			CopyTruncate: copyTruncate,
			Sequential:   sequential,
		}
		t.Cleanup(func() { _ = rotator.Close() })

		logger := log.New(&rotator, "", 0)

		expected := make([][]byte, 0, tc.totalLines)
		baseStr := string(filetest.TokenWithLength(46)) // + ' 123'
		for i := 0; i < tc.totalLines; i++ {
			expected = append(expected, []byte(fmt.Sprintf("%s %3d", baseStr, i)))
		}

		require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
		defer func() {
			require.NoError(t, operator.Stop())
		}()

		for _, message := range expected {
			logger.Println(string(message))
			time.Sleep(tc.writeInterval)
		}

		received := make([][]byte, 0, tc.totalLines)
		for i := 0; i < tc.totalLines; i++ {
			received = append(received, sink.NextToken(t))
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
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temp1 := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp1, "testlog1\n")
	temp1.Close()

	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))

	// Wait until all goroutines are finished before renaming
	operator.wg.Wait()
	err := os.Rename(temp1.Name(), fmt.Sprintf("%s.2", temp1.Name()))
	require.NoError(t, err)

	operator.poll(context.Background())
	sink.ExpectNoCalls(t)
}

func TestTrackMovedAwayFiles(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Moving files while open is unsupported on Windows")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temp1 := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp1, "testlog1\n")
	temp1.Close()

	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))

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
	filetest.WriteString(t, movedFile, "testlog2\n")
	operator.poll(context.Background())

	sink.ExpectToken(t, []byte("testlog2"))
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
	operator, sink := testManager(t, cfg)

	originalFile := filetest.OpenTemp(t, tempDir)
	orginalName := originalFile.Name()
	filetest.WriteString(t, originalFile, "testlog1\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte("testlog1"))
	filetest.WriteString(t, originalFile, "testlog2\n")
	originalFile.Close()

	newDir := fmt.Sprintf("%s%s", tempDir[:len(tempDir)-1], "_new/")
	require.NoError(t, os.Mkdir(newDir, 0777))
	movedFileName := fmt.Sprintf("%s%s", newDir, "newfile.log")

	require.NoError(t, os.Rename(orginalName, movedFileName))

	newFile, err := os.OpenFile(orginalName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	filetest.WriteString(t, newFile, "testlog3\n")

	sink.ExpectTokens(t, []byte("testlog2"), []byte("testlog3"))
}

// When a file it rotated out of pattern via move/create, we should
// detect that our old handle is still valid attempt to read from it.
func TestRotatedOutOfPatternMoveCreate(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Moving files while open is unsupported on Windows")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig()
	cfg.Include = append(cfg.Include, fmt.Sprintf("%s/*.log1", tempDir))
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	originalFile := filetest.OpenTempWithPattern(t, tempDir, "*.log1")
	originalFileName := originalFile.Name()

	filetest.WriteString(t, originalFile, "testlog1\n")
	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))

	// write more log, before next poll() begins
	filetest.WriteString(t, originalFile, "testlog2\n")

	// move the file so it no longer matches
	require.NoError(t, originalFile.Close())
	require.NoError(t, os.Rename(originalFileName, originalFileName+".old"))

	newFile := filetest.OpenFile(t, originalFileName)
	_, err := newFile.Write([]byte("testlog4\ntestlog5\n"))
	require.NoError(t, err)

	// poll again
	operator.poll(context.Background())

	// expect remaining log from old file as well as all from new file
	sink.ExpectTokens(t, []byte("testlog2"), []byte("testlog4"), []byte("testlog5"))
}

// When a file it rotated out of pattern via copy/truncate, we should
// detect that our old handle is stale and not attempt to read from it.
func TestRotatedOutOfPatternCopyTruncate(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig()
	cfg.Include = append(cfg.Include, fmt.Sprintf("%s/*.log1", tempDir))
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	originalFile := filetest.OpenTempWithPattern(t, tempDir, "*.log1")
	filetest.WriteString(t, originalFile, "testlog1\n")
	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog1"))

	// write more log, before next poll() begins
	filetest.WriteString(t, originalFile, "testlog2\n")
	// copy the file to another file i.e. rotate, out of pattern
	newFile := filetest.OpenTempWithPattern(t, tempDir, "*.log2")
	_, err := originalFile.Seek(0, 0)
	require.NoError(t, err)
	_, err = io.Copy(newFile, originalFile)
	require.NoError(t, err)

	_, err = originalFile.Seek(0, 0)
	require.NoError(t, err)
	require.NoError(t, originalFile.Truncate(0))
	_, err = originalFile.Write([]byte("testlog4\ntestlog5\n"))
	require.NoError(t, err)

	// poll again
	operator.poll(context.Background())

	sink.ExpectTokens(t, []byte("testlog4"), []byte("testlog5"))
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
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temp1 := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp1, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	sink.ExpectTokens(t, []byte("testlog1"), []byte("testlog2"))

	require.NoError(t, temp1.Truncate(0))
	_, err := temp1.Seek(0, 0)
	require.NoError(t, err)

	filetest.WriteString(t, temp1, "testlog3\n")
	operator.poll(context.Background())
	sink.ExpectToken(t, []byte("testlog3"))
	sink.ExpectNoCalls(t)
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
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()

	temp1 := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp1, "testlog1\ntestlog2\n")

	operator.poll(context.Background())
	sink.ExpectTokens(t, []byte("testlog1"), []byte("testlog2"))
	operator.wg.Wait() // wait for all goroutines to finish

	// Copy the first file to a new file, and add another log
	temp2 := filetest.OpenTemp(t, tempDir)
	_, err := io.Copy(temp2, temp1)
	require.NoError(t, err)

	// Truncate original file
	require.NoError(t, temp1.Truncate(0))
	_, err = temp1.Seek(0, 0)
	require.NoError(t, err)

	// Write to original and new file
	filetest.WriteString(t, temp2, "testlog3\n")
	filetest.WriteString(t, temp1, "testlog4\n")

	// Expect both messages to come through
	operator.poll(context.Background())
	sink.ExpectTokens(t, []byte("testlog3"), []byte("testlog4"))
}

func TestFileMovedWhileOff_BigFiles(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	persister := testutil.NewUnscopedMockPersister()

	log1 := filetest.TokenWithLength(1001)
	log2 := filetest.TokenWithLength(1002)
	log3 := filetest.TokenWithLength(1003)

	temp := filetest.OpenTemp(t, tempDir)
	tempName := temp.Name()
	filetest.WriteString(t, temp, string(log1)+"\n")

	// Run the operator to read the first log
	require.NoError(t, operator.Start(persister))
	sink.ExpectToken(t, log1)
	require.NoError(t, operator.Stop())

	// Write one more log to the original file
	filetest.WriteString(t, temp, string(log2)+"\n")
	require.NoError(t, temp.Close())

	// Rename the file and open another file in the same location
	require.NoError(t, os.Rename(tempName, fmt.Sprintf("%s2", tempName)))

	// Write a different log to the new file
	temp2 := filetest.ReopenTemp(t, tempName)
	filetest.WriteString(t, temp2, string(log3)+"\n")

	// Expect the message written to the new log to come through
	operator2, sink2 := testManager(t, cfg)
	require.NoError(t, operator2.Start(persister))
	sink2.ExpectTokens(t, log2, log3)
	require.NoError(t, operator2.Stop())
}
