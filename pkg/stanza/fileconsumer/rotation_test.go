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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

const windowsOS = "windows"

func TestCopyTruncate(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.StartAt = "beginning"
	cfg.PollInterval = 10 * time.Millisecond
	operator, sink := testManager(t, cfg)

	getMessage := func(f, k, m int) string { return fmt.Sprintf("file %d-%d, message %d", f, k, m) }
	fileName := func(f, k int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.rot%d.log", f, k)) }
	baseFileName := func(f int) string { return filepath.Join(tempDir, fmt.Sprintf("file%d.log", f)) }

	numFiles := 3
	numMessages := 300
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

			file := filetest.OpenFile(t, baseFileName(fn))
			for rotationNum := 0; rotationNum < numRotations; rotationNum++ {
				for messageNum := 0; messageNum < numMessages; messageNum++ {
					filetest.WriteString(t, file, getMessage(fn, rotationNum, messageNum)+"\n")
					time.Sleep(10 * time.Millisecond)
				}
				assert.NoError(t, file.Sync())
				_, err := file.Seek(0, 0)
				assert.NoError(t, err)
				dst := filetest.OpenFile(t, fileName(fn, rotationNum))
				_, err = io.Copy(dst, file)
				assert.NoError(t, err)
				assert.NoError(t, dst.Close())
				assert.NoError(t, file.Truncate(0))
				_, err = file.Seek(0, 0)
				assert.NoError(t, err)
			}
		}(fileNum)
	}

	sink.ExpectTokens(t, expected...)
	wg.Wait()
}

func TestMoveCreate(t *testing.T) {
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
					time.Sleep(10 * time.Millisecond)
				}
				assert.NoError(t, file.Close())
				assert.NoError(t, os.Rename(baseFileName(fn), fileName(fn, rotationNum)))
			}
		}(fileNum)
	}

	sink.ExpectTokens(t, expected...)
	wg.Wait()
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
	err := os.Mkdir(newDir, 0o777)
	require.NoError(t, err)
	newFileName := fmt.Sprintf("%s%s", newDir, "newfile.log")

	err = os.Rename(temp1.Name(), newFileName)
	require.NoError(t, err)

	movedFile, err := os.OpenFile(newFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
	core, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	operator.set.Logger = logger

	originalFile := filetest.OpenTemp(t, tempDir)
	originalName := originalFile.Name()
	filetest.WriteString(t, originalFile, "testlog1\n")

	require.NoError(t, operator.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	sink.ExpectToken(t, []byte("testlog1"))
	filetest.WriteString(t, originalFile, "testlog2\n")
	originalFile.Close()

	newDir := fmt.Sprintf("%s%s", tempDir[:len(tempDir)-1], "_new/")
	require.NoError(t, os.Mkdir(newDir, 0o777))
	movedFileName := fmt.Sprintf("%s%s", newDir, "newfile.log")

	require.NoError(t, os.Rename(originalName, movedFileName))

	newFile, err := os.OpenFile(originalName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	filetest.WriteString(t, newFile, "testlog3\n")

	sink.ExpectTokens(t, []byte("testlog2"), []byte("testlog3"))

	// verify that proper logging has taken place
	allLogs := observedLogs.All()
	foundLog := false
	for _, actualLog := range allLogs {
		if actualLog.Message == "File has been rotated(moved)" {
			foundLog = true
		}
	}
	assert.True(t, foundLog)
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
	core, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	operator.set.Logger = logger

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

	// verify that proper logging has taken place
	allLogs := observedLogs.All()
	expectedLogs := map[string]string{
		"File has been rotated(moved)": "",
		"Reading lost file":            "",
	}
	foundLogs := 0
	for _, actualLog := range allLogs {
		if _, ok := expectedLogs[actualLog.Message]; ok {
			foundLogs++
		}
	}
	assert.Equal(t, 2, foundLogs)
}

// When a file it rotated out of pattern via copy/truncate, we should
// detect that our old handle is stale and not attempt to read from it.
func TestRotatedOutOfPatternCopyTruncate(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig()
	cfg.Include = append(cfg.Include, fmt.Sprintf("%s/*.log1", tempDir))
	cfg.StartAt = "beginning"
	operator, sink := testManager(t, cfg)
	operator.persister = testutil.NewUnscopedMockPersister()
	core, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	operator.set.Logger = logger

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

	// verify that proper logging has taken place
	allLogs := observedLogs.All()
	foundLog := false
	for _, actualLog := range allLogs {
		if actualLog.Message == "File has been rotated(truncated)" {
			foundLog = true
		}
	}
	assert.True(t, foundLog)
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
	core, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	operator.set.Logger = logger

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

	// verify that proper logging has taken place
	allLogs := observedLogs.All()
	foundLog := false
	for _, actualLog := range allLogs {
		if actualLog.Message == "File has been rotated(truncated)" {
			foundLog = true
		}
	}
	assert.True(t, foundLog)
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
