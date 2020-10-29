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

package stanzareceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/observiq/nanojack"
	"github.com/observiq/stanza/entry"
	"github.com/observiq/stanza/pipeline"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
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

func unmarshalConfig(t *testing.T, pipelineYaml string) pipeline.Config {
	pipelineCfg := pipeline.Config{}
	require.NoError(t, yaml.Unmarshal([]byte(pipelineYaml), &pipelineCfg))
	return pipelineCfg
}

func expectNLogs(sink *exportertest.SinkLogsExporter, expected int) func() bool {
	return func() bool { return sink.LogRecordsCount() == expected }
}

func TestReadStaticFile(t *testing.T) {
	t.Parallel()

	expectedTimestamp, _ := time.ParseInLocation("2006-01-02", "2020-08-25", time.Local)

	e1 := entry.New()
	e1.Timestamp = expectedTimestamp
	e1.Severity = entry.Info
	e1.Set(entry.NewRecordField("msg"), "Something routine")
	e1.AddLabel("file_name", "simple.log")

	e2 := entry.New()
	e2.Timestamp = expectedTimestamp
	e2.Severity = entry.Error
	e2.Set(entry.NewRecordField("msg"), "Something bad happened!")
	e2.AddLabel("file_name", "simple.log")

	e3 := entry.New()
	e3.Timestamp = expectedTimestamp
	e3.Severity = entry.Debug
	e3.Set(entry.NewRecordField("msg"), "Some details...")
	e3.AddLabel("file_name", "simple.log")

	expectedLogs := []pdata.Logs{convert(e1), convert(e2), convert(e3)}

	f := NewFactory()
	sink := &exportertest.SinkLogsExporter{}
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Pipeline = unmarshalConfig(t, `
- type: file_input
  include: [testdata/simple.log]
  start_at: beginning
- type: regex_parser
  regex: '^(?P<ts>\d{4}-\d{2}-\d{2}) (?P<sev>[A-Z]+) (?P<msg>[^\n]+)'
  timestamp:
    parse_from: ts
    layout: '%Y-%m-%d'
  severity:
    parse_from: sev`)
	rcvr, err := f.CreateLogsReceiver(context.Background(), params, cfg, sink)
	require.NoError(t, err, "failed to create receiver")

	require.NoError(t, rcvr.Start(context.Background(), &testHost{t: t}))
	require.Eventually(t, expectNLogs(sink, 3), time.Second, time.Millisecond)
	require.Equal(t, expectedLogs, sink.AllLogs())
	require.NoError(t, rcvr.Shutdown(context.Background()))
}

func TestReadRotatingFiles(t *testing.T) {
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1382
	if runtime.GOOS == "windows" {
		t.Skip()
	}

	tests := []rotationTest{
		{
			name:         "CopyTruncateTimestamped",
			copyTruncate: true,
			sequential:   false,
		},
		{
			name:         "MoveCreateTimestamped",
			copyTruncate: false,
			sequential:   false,
		},
		{
			name:         "CopyTruncateSequential",
			copyTruncate: true,
			sequential:   true,
		},
		{
			name:         "MoveCreateSequential",
			copyTruncate: false,
			sequential:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.Run)
	}
}

type rotationTest struct {
	name         string
	copyTruncate bool
	sequential   bool
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
	logger := newRotatingLogger(t, tempDir, 100, 1, rt.copyTruncate, rt.sequential)
	numLogs := 300

	// Build input lines and expected outputs
	lines := make([]string, numLogs)
	expectedLogs := make([]pdata.Logs, numLogs)
	expectedTimestamp, _ := time.ParseInLocation("2006-01-02", "2020-08-25", time.Local)
	for i := 0; i < numLogs; i++ {
		msg := fmt.Sprintf("This is a simple log line with the number %3d", i)
		lines[i] = fmt.Sprintf("2020-08-25 %s", msg)

		e := entry.New()
		e.Timestamp = expectedTimestamp
		e.Set(entry.NewRecordField("msg"), msg)
		expectedLogs[i] = convert(e)
	}

	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Pipeline = unmarshalConfig(t, fmt.Sprintf(`
  - type: file_input
    include: [%s/*]
    include_file_name: false
    start_at: beginning
    poll_interval: 10ms
  - type: regex_parser
    regex: '^(?P<ts>\d{4}-\d{2}-\d{2}) (?P<msg>[^\n]+)'
    timestamp:
      parse_from: ts
      layout: '%%Y-%%m-%%d'`, tempDir))

	rcvr, err := f.CreateLogsReceiver(context.Background(), params, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(context.Background(), &testHost{t: t}))

	for _, line := range lines {
		logger.Print(line)
		time.Sleep(time.Millisecond)
	}

	require.Eventually(t, expectNLogs(sink, numLogs), 2*time.Second, time.Millisecond)
	require.ElementsMatch(t, expectedLogs, sink.AllLogs())
	require.NoError(t, rcvr.Shutdown(context.Background()))
}

func newRotatingLogger(t *testing.T, tempDir string, maxLines, maxBackups int, copyTruncate, sequential bool) *log.Logger {
	path := filepath.Join(tempDir, "test.log")
	rotator := &nanojack.Logger{
		Filename:     path,
		MaxLines:     maxLines,
		MaxBackups:   maxBackups,
		CopyTruncate: copyTruncate,
		Sequential:   sequential,
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
