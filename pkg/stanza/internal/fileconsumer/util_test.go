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
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/observiq/nanojack"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newDefaultConfig(tempDir string) *Config {
	cfg := NewConfig()
	cfg.PollInterval = helper.Duration{Duration: 200 * time.Millisecond}
	cfg.StartAt = "beginning"
	cfg.Include = []string{fmt.Sprintf("%s/*", tempDir)}
	return cfg
}

func emitOnChan(received chan []byte) EmitFunc {
	return func(_ context.Context, _ *FileAttributes, token []byte) {
		received <- token
	}
}

type emitParams struct {
	attrs *FileAttributes
	token []byte
}

func newTestScenario(t *testing.T, cfgMod func(*Config)) (*Input, chan *emitParams, string) {
	emitChan := make(chan *emitParams, 100)
	input, tempDir := newTestScenarioWithChan(t, cfgMod, emitChan)
	return input, emitChan, tempDir
}

func newTestScenarioWithChan(t *testing.T, cfgMod func(*Config), emitChan chan *emitParams) (*Input, string) {
	tempDir := t.TempDir()
	cfg := newDefaultConfig(tempDir)
	if cfgMod != nil {
		cfgMod(cfg)
	}

	input, err := cfg.Build(testutil.Logger(t), func(_ context.Context, attrs *FileAttributes, token []byte) {
		emitChan <- &emitParams{attrs, token}
	})
	require.NoError(t, err)

	return input, tempDir
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

func tokenWithLength(length int) []byte {
	charset := "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return b
}

func waitForEmit(t *testing.T, c chan *emitParams) *emitParams {
	select {
	case call := <-c:
		return call
	case <-time.After(3 * time.Second):
		require.FailNow(t, "Timed out waiting for message")
		return nil
	}
}

func waitForNTokens(t *testing.T, c chan *emitParams, n int) [][]byte {
	emitChan := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		select {
		case call := <-c:
			emitChan = append(emitChan, call.token)
		case <-time.After(3 * time.Second):
			require.FailNow(t, "Timed out waiting for message")
			return nil
		}
	}
	return emitChan
}

func waitForToken(t *testing.T, c chan *emitParams, expected []byte) {
	select {
	case call := <-c:
		require.Equal(t, expected, call.token)
	case <-time.After(3 * time.Second):
		require.FailNow(t, "Timed out waiting for token", expected)
	}
}

func waitForTokens(t *testing.T, c chan *emitParams, expected [][]byte) {
	actual := make([][]byte, 0, len(expected))
LOOP:
	for {
		select {
		case call := <-c:
			actual = append(actual, call.token)
		case <-time.After(3 * time.Second):
			break LOOP
		}
	}

	require.ElementsMatch(t, expected, actual)
}

func expectNoTokens(t *testing.T, c chan *emitParams) {
	expectNoTokensUntil(t, c, 200*time.Millisecond)
}

func expectNoTokensUntil(t *testing.T, c chan *emitParams, d time.Duration) {
	select {
	case token := <-c:
		require.FailNow(t, "Received unexpected message", "Message: %s", token)
	case <-time.After(d):
	}
}
