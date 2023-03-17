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
		operator := buildTestManagerWithEmit(t, cfg, emitCalls)

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

//Reading a file with a multi-cycle should work well
func TestTenPollCycleForOneFile(t *testing.T) {
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.FingerprintSize = 16
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	file := openTemp(t,tempDir)
	for i:=0;i<10;i++ {
		content := fmt.Sprintf("aaaaaaaaaaaaaaaa%d", i)
		writeString(t,file,content+"\n")
		operator.poll(context.Background())
		waitForToken(t,emitCalls,[]byte(content))
	}

}

//If the rotation is done with the same fingerprint between the pre-rotated file and  the post-rotated file  ,
//and the file does not grow rapidly after the  rotation  , there should be no data loss
func TestSlowFileSameFingerprintAfterRotation(t *testing.T){
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig()
	cfg.FingerprintSize = 16
	cfg.StartAt = "beginning"
	cfg.Include = append(cfg.Include,fmt.Sprintf("%s/*.log",tempDir))

	operator, emitCalls := buildTestManager(t, cfg)
	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	originalFile := openTempWithPattern(t, tempDir,"*.log")
	writeString(t,originalFile,"aaaaaaaaaaaaaaaa\n")
	writeString(t,originalFile,"123\n")
	writeString(t,originalFile,"456\n")

	waitForToken(t,emitCalls,[]byte("aaaaaaaaaaaaaaaa"))
	waitForToken(t,emitCalls,[]byte("123"))
	waitForToken(t,emitCalls,[]byte("456"))

	//The rotation is performed  and the rotation file is excluded
	rotationFile := openTempWithPattern(t, tempDir,"*.log.1")
	_, err := originalFile.Seek(0, 0)
	require.NoError(t,err)
	_, err = io.Copy(rotationFile, originalFile)
	require.NoError(t, err)
	rotationFileInfo, _ := rotationFile.Stat()
	originalFileInfo, _ := originalFile.Stat()
	require.Equal(t,rotationFileInfo.Size(), originalFileInfo.Size())

	require.NoError(t, originalFile.Truncate(0))
	_, err = originalFile.Seek(0, 0)
	require.NoError(t, err)

	//The first 16 bytes indicate that the file after rotation has the same fingerprint as the file before rotation
	//Write a total of 21 bytes to indicate that the rotated file will be written slowly before the next read
	writeString(t,originalFile,"aaaaaaaaaaaaaaaa\n")
	writeString(t,originalFile,"bbb\n")
	writeString(t,originalFile,"cc\n")

	waitForToken(t,emitCalls,[]byte("aaaaaaaaaaaaaaaa"))
	waitForToken(t,emitCalls,[]byte("bbb"))
	waitForToken(t,emitCalls,[]byte("cc"))
}

//If the rotation is done with the same fingerprint between the pre-rotated file and  the post-rotated file  ,
//and the file does grows rapidly after the  rotation  , there will be data loss
//Note: there is no metric/log yet to tell people to  increase the fingerprint size
func TestFastFileSameFingerprintAfterRotation(t *testing.T){
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig()
	cfg.FingerprintSize = 16
	cfg.StartAt = "beginning"
	cfg.Include = append(cfg.Include,fmt.Sprintf("%s/*.log",tempDir))

	operator, emitCalls := buildTestManager(t, cfg)
	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	originalFile := openTempWithPattern(t, tempDir,"*.log")
	writeString(t,originalFile,"aaaaaaaaaaaaaaaa\n")
	writeString(t,originalFile,"123\n")
	writeString(t,originalFile,"456\n")

	waitForToken(t,emitCalls,[]byte("aaaaaaaaaaaaaaaa"))
	waitForToken(t,emitCalls,[]byte("123"))
	waitForToken(t,emitCalls,[]byte("456"))

	//The rotation is performed  and the rotation file is excluded
	rotationFile := openTempWithPattern(t, tempDir,"*.log.1")
	_, err := originalFile.Seek(0, 0)
	require.NoError(t,err)
	_, err = io.Copy(rotationFile, originalFile)
	require.NoError(t, err)
	rotationFileInfo, _ := rotationFile.Stat()
	originalFileInfo, _ := originalFile.Stat()
	require.Equal(t,rotationFileInfo.Size(), originalFileInfo.Size())

	require.NoError(t, originalFile.Truncate(0))
	_, err = originalFile.Seek(0, 0)
	require.NoError(t, err)

	//The first 16 bytes indicate that the file after rotation has the same fingerprint as the file before rotation
	//Write a total of 25 bytes to indicate that the rotated file will be written quickly before the next read
	writeString(t,originalFile,"aaaaaaaaaaaaaaaa\n")
	writeString(t,originalFile,"bbb\n")
	writeString(t,originalFile,"ccc\n")
	writeString(t,originalFile,"ddd\n")

	//data lost before ddd
	waitForToken(t,emitCalls,[]byte("ddd"))

}

//Always exclude the same file
func TestTwoSameFingerprintFileIngestOnyOneFileContent(t *testing.T){
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.FingerprintSize = 16
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	require.NoError(t, operator.Start(testutil.NewMockPersister("test")))
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	file1 := openTempWithPattern(t,tempDir,"*.log1")
	file2 := openTempWithPattern(t,tempDir,"*.log2")


	fileExcluded := file2.Name()

	for i:=0; i<10;i++  {
		content := fmt.Sprintf("aaaaaaaaaaaaaaaa%d", i)
		writeString(t,file1, content+"\n")
		if i == 0 {
			writeString(t,file2, content+"\n")
			waitForToken(t,emitCalls,[]byte(content))
			if _,ok :=operator.excludePaths[file1.Name()]; ok{
				fileExcluded = file1.Name()
			}
		}else{
			writeString(t,file2, content+"zzzz\n")
			if _, ok := operator.excludePaths[file1.Name()]; ok {
				waitForToken(t,emitCalls,[]byte(content+"zzzz"))
			}else{
				waitForToken(t,emitCalls,[]byte(content))
			}
		}
		require.Equal(t,1, len(operator.excludePaths))
		require.Contains(t, operator.excludePaths, fileExcluded)

		expectNoTokens(t,emitCalls)
	}
}


//If the rotation is done with the same fingerprint between the pre-rotated file and  the post-rotated file  ,
//the file does not grow rapidly after the  rotation  , and the pre-rotated file still append data
//there should be no data loss and no data reading chaos
func TestSlowFileSameFingerprintAfterRotationWithoutExclude(t *testing.T){
	if runtime.GOOS == windowsOS {
		t.Skip("Rotation tests have been flaky on Windows. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16331")
	}
	t.Parallel()

	tempDir := t.TempDir()
	cfg := NewConfig().includeDir(tempDir)
	cfg.FingerprintSize = 16
	cfg.StartAt = "beginning"
	operator, emitCalls := buildTestManager(t, cfg)
	operator.persister = testutil.NewMockPersister("test")
	defer func() {
		require.NoError(t, operator.Stop())
	}()

	originalFile := openTemp(t, tempDir)
	writeString(t,originalFile,"aaaaaaaaaaaaaaaa\n")
	writeString(t,originalFile,"123\n")
	writeString(t,originalFile,"456\n")

	operator.poll(context.Background())
	waitForToken(t,emitCalls,[]byte("aaaaaaaaaaaaaaaa"))
	waitForToken(t,emitCalls,[]byte("123"))
	waitForToken(t,emitCalls,[]byte("456"))
	operator.wg.Wait()

	//The rotation is performed  and the rotation file is included
	rotationFile := openTemp(t, tempDir)
	_, err := originalFile.Seek(0, 0)
	require.NoError(t,err)
	_, err = io.Copy(rotationFile, originalFile)
	require.NoError(t, err)
	rotationFileInfo, _ := rotationFile.Stat()
	originalFileInfo, _ := originalFile.Stat()
	require.Equal(t,rotationFileInfo.Size(), originalFileInfo.Size())

	require.NoError(t, originalFile.Truncate(0))
	_, err = originalFile.Seek(0, 0)
	require.NoError(t, err)

	//write 19 bytes to indicate that rotation File write slow before next read
	writeString(t,originalFile,"aaaaaaaaaaaaaaaa\n")
	writeString(t,originalFile,"bbb\n")

	operator.poll(context.Background())
	waitForToken(t,emitCalls,[]byte("aaaaaaaaaaaaaaaa"))
	waitForToken(t,emitCalls,[]byte("bbb"))
	operator.wg.Wait()


	//Write data to the rotation file.
	//As the rotation file is excluded due to the fingerprint check , there should be no data output
	require.Contains(t,operator.excludePaths,rotationFile.Name())
	writeString(t,rotationFile,"7891011121314151617\n")
	operator.poll(context.Background())
	expectNoTokens(t,emitCalls)
	operator.wg.Wait()

	writeString(t,originalFile,"ccc\n")
	writeString(t,originalFile,"ddd\n")

	//no chaos
	operator.poll(context.Background())
	waitForToken(t,emitCalls,[]byte("ccc"))
	waitForToken(t,emitCalls,[]byte("ddd"))
	operator.wg.Wait()

}
