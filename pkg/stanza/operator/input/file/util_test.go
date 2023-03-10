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
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newDefaultConfig(tempDir string) *Config {
	cfg := NewConfigWithID("testfile")
	cfg.PollInterval = 200 * time.Millisecond
	cfg.StartAt = "beginning"
	cfg.Include = []string{fmt.Sprintf("%s/*", tempDir)}
	cfg.OutputIDs = []string{"fake"}
	return cfg
}

func newTestFileOperator(t *testing.T, cfgMod func(*Config)) (*Input, chan *entry.Entry, string) {
	fakeOutput := testutil.NewFakeOutput(t)

	tempDir := t.TempDir()

	cfg := newDefaultConfig(tempDir)
	if cfgMod != nil {
		cfgMod(cfg)
	}
	op, err := cfg.Build(testutil.Logger(t))
	require.NoError(t, err)

	err = op.SetOutputs([]operator.Operator{fakeOutput})
	require.NoError(t, err)

	return op.(*Input), fakeOutput.Received, tempDir
}

func openFile(tb testing.TB, path string) *os.File {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(tb, err)
	tb.Cleanup(func() { _ = file.Close() })
	return file
}

func openTemp(t testing.TB, tempDir string) *os.File {
	return openTempWithPattern(t, tempDir, "")
}

func openTempWithPattern(t testing.TB, tempDir, pattern string) *os.File {
	file, err := os.CreateTemp(tempDir, pattern)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func writeString(t testing.TB, file *os.File, s string) {
	_, err := file.WriteString(s)
	require.NoError(t, err)
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
	case <-time.After(3 * time.Second):
		require.FailNow(t, "Timed out waiting for message")
		return nil
	}
}

func waitForMessage(t *testing.T, c chan *entry.Entry, expected string) {
	select {
	case e := <-c:
		require.Equal(t, expected, e.Body.(string))
	case <-time.After(3 * time.Second):
		require.FailNow(t, "Timed out waiting for message", expected)
	}
}

func waitForByteMessage(t *testing.T, c chan *entry.Entry, expected []byte) {
	select {
	case e := <-c:
		require.Equal(t, expected, e.Body.([]byte))
	case <-time.After(3 * time.Second):
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
		case <-time.After(3 * time.Second):
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
