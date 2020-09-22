package stanzareceiver

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

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/observiq/nanojack"
	"github.com/observiq/stanza/pipeline"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap/zaptest"
	"gopkg.in/yaml.v2"
)

type testHost struct {
	component.Host
	t *testing.T
}

// ReportFatalError causes the test to be run to fail.
func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("receiver reported a fatal error: %v", err)
}

var _ component.Host = (*testHost)(nil)

func unmarshal(t *testing.T, pipelineYaml string) pipeline.Config {
	pipelineCfg := pipeline.Config{}
	require.NoError(t, yaml.Unmarshal([]byte(pipelineYaml), &pipelineCfg))
	return pipelineCfg
}

func expectNLogs(sink *exportertest.SinkLogsExporter, expected int) func() bool {
	return func() bool { return sink.LogRecordsCount() == expected }
}

func TestReadStaticFile(t *testing.T) {
	t.Parallel()
	f := NewFactory()
	sink := &exportertest.SinkLogsExporter{}
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Pipeline = unmarshal(t, `
- type: file_input
  include: [testdata/simple.log]
  start_at: beginning`)

	rcvr, err := f.CreateLogsReceiver(context.Background(), params, cfg, sink)
	require.NoError(t, err, "failed to create receiver")

	require.NoError(t, rcvr.Start(context.Background(), &testHost{t: t}))
	require.Eventually(t, expectNLogs(sink, 3), time.Second, time.Millisecond)
	require.NoError(t, rcvr.Shutdown(context.Background()))
}

func TestReadRotatingFiles(t *testing.T) {
	tests := []rotationTest{
		{"CopyTruncate", true},
		{"NoCopyTruncate", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.Run)
	}
}

type rotationTest struct {
	name         string
	copyTruncate bool
}

func (rt *rotationTest) Run(t *testing.T) {
	t.Parallel()
	f := NewFactory()
	sink := &exportertest.SinkLogsExporter{}
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	tempDir := newTempDir(t)

	// With a max of 100 logs per file and 1 backup file, rotation will occur
	// when more than 100 logs are written, and deletion after 200 logs are written.
	// Write 300 and validate that we got the all despite rotation and deletion.
	logger := newRotatingLogger(t, tempDir, 100, 1, rt.copyTruncate)
	numLogs := 300

	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Pipeline = unmarshal(t, fmt.Sprintf(`
- type: file_input
  include: [%s/*]
  start_at: beginning
  poll_interval: 10ms`, tempDir))

	rcvr, err := f.CreateLogsReceiver(context.Background(), params, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(context.Background(), &testHost{t: t}))

	for i := 0; i < numLogs; i++ {
		logger.Printf("This is a simple log line with the number %3d", i)
		time.Sleep(time.Millisecond)
	}

	require.Eventually(t, expectNLogs(sink, numLogs), 2*time.Second, time.Millisecond)
	require.NoError(t, rcvr.Shutdown(context.Background()))
}

func newRotatingLogger(t *testing.T, tempDir string, maxLines, maxBackups int, copyTruncate bool) *log.Logger {
	path := filepath.Join(tempDir, "test.log")
	rotator := &nanojack.Logger{
		Filename:     path,
		MaxLines:     maxLines,
		MaxBackups:   maxBackups,
		CopyTruncate: copyTruncate,
	}

	t.Cleanup(func() { _ = rotator.Close() })

	return log.New(rotator, "", 0)
}

func newTempDir(t *testing.T) string {
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	t.Logf("Temp Dir: %s", tempDir)

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return tempDir
}
