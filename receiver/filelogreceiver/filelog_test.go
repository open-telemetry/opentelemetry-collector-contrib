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

package filelogreceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/observiq/nanojack"
	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 1)

	assert.Equal(t, testdataConfigYamlAsMap(), cfg.Receivers["filelog"])
}

func TestCreateWithInvalidInputConfig(t *testing.T) {
	t.Parallel()

	cfg := testdataConfigYamlAsMap()
	cfg.Input["include"] = "not an array"

	_, err := NewFactory().CreateLogsReceiver(
		context.Background(),
		component.ReceiverCreateParams{
			Logger: zaptest.NewLogger(t),
		},
		cfg,
		new(consumertest.LogsSink),
	)
	require.Error(t, err, "receiver creation should fail if given invalid input config")
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

	expectedLogs := []pdata.Logs{
		stanza.Convert(e1),
		stanza.Convert(e2),
		stanza.Convert(e3),
	}

	f := NewFactory()
	sink := new(consumertest.LogsSink)
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	rcvr, err := f.CreateLogsReceiver(context.Background(), params, testdataConfigYamlAsMap(), sink)
	require.NoError(t, err, "failed to create receiver")

	dir, err := os.Getwd()
	require.NoError(t, err)
	t.Logf("Working Directory: %s", dir)

	require.NoError(t, rcvr.Start(context.Background(), &testHost{t: t}))
	require.Eventually(t, expectNLogs(sink, 3), time.Second, time.Millisecond)
	require.Equal(t, expectedLogs, sink.AllLogs())
	require.NoError(t, rcvr.Shutdown(context.Background()))
}

func TestReadRotatingFiles(t *testing.T) {

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
	sink := new(consumertest.LogsSink)
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	tempDir := newTempDir(t)

	// With a max of 100 logs per file and 1 backup file, rotation will occur
	// when more than 100 logs are written, and deletion when more than 200 are written.
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
		expectedLogs[i] = stanza.Convert(e)
	}

	rcvr, err := f.CreateLogsReceiver(context.Background(), params, testdataRotateTestYamlAsMap(tempDir), sink)
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

func expectNLogs(sink *consumertest.LogsSink, expected int) func() bool {
	return func() bool { return sink.LogRecordsCount() == expected }
}

type testHost struct {
	component.Host
	t *testing.T
}

var _ component.Host = (*testHost)(nil)

// ReportFatalError causes the test to be run to fail.
func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("receiver reported a fatal error: %v", err)
}

func testdataConfigYamlAsMap() *FileLogConfig {
	return &FileLogConfig{
		BaseConfig: stanza.BaseConfig{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "filelog",
				NameVal: "filelog",
			},
			Operators: stanza.OperatorConfigs{
				map[string]interface{}{
					"type":  "regex_parser",
					"regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$",
					"severity": map[interface{}]interface{}{
						"parse_from": "sev",
					},
					"timestamp": map[interface{}]interface{}{
						"layout":     "%Y-%m-%d",
						"parse_from": "time",
					},
				},
			},
		},
		Input: stanza.InputConfig{
			"include": []interface{}{
				"testdata/simple.log",
			},
			"start_at": "beginning",
		},
	}
}

func testdataRotateTestYamlAsMap(tempDir string) *FileLogConfig {
	return &FileLogConfig{
		BaseConfig: stanza.BaseConfig{
			ReceiverSettings: configmodels.ReceiverSettings{
				TypeVal: "filelog",
				NameVal: "filelog",
			},
			Operators: stanza.OperatorConfigs{
				map[string]interface{}{
					"type":  "regex_parser",
					"regex": "^(?P<ts>\\d{4}-\\d{2}-\\d{2}) (?P<msg>[^\n]+)",
					"timestamp": map[interface{}]interface{}{
						"layout":     "%Y-%m-%d",
						"parse_from": "ts",
					},
				},
			},
		},
		Input: stanza.InputConfig{
			"type": "file_input",
			"include": []interface{}{
				fmt.Sprintf("%s/*", tempDir),
			},
			"include_file_name": false,
			"poll_interval":     "10ms",
			"start_at":          "beginning",
		},
	}
}
