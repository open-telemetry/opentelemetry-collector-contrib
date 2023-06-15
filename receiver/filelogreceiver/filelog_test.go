// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/observiq/nanojack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/json"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewID("filelog").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.NoError(t, component.ValidateConfig(cfg))
	assert.Equal(t, testdataConfigYaml(), cfg)
}

func TestCreateWithInvalidInputConfig(t *testing.T) {
	t.Parallel()

	cfg := testdataConfigYaml()
	cfg.InputConfig.StartAt = "middle"

	_, err := NewFactory().CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		new(consumertest.LogsSink),
	)
	require.Error(t, err, "receiver creation should fail if given invalid input config")
}

func TestReadStaticFile(t *testing.T) {
	t.Parallel()

	expectedTimestamp, _ := time.ParseInLocation("2006-01-02", "2020-08-25", time.Local)

	f := NewFactory()
	sink := new(consumertest.LogsSink)
	cfg := testdataConfigYaml()

	converter := adapter.NewConverter(zap.NewNop())
	converter.Start()
	defer converter.Stop()

	var wg sync.WaitGroup
	wg.Add(1)
	go consumeNLogsFromConverter(converter.OutChannel(), 3, &wg)

	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	// Build the expected set by using adapter.Converter to translate entries
	// to pdata Logs.
	queueEntry := func(t *testing.T, c *adapter.Converter, msg string, severity entry.Severity) {
		e := entry.New()
		e.Timestamp = expectedTimestamp
		require.NoError(t, e.Set(entry.NewBodyField("msg"), msg))
		e.Severity = severity
		e.AddAttribute("file_name", "simple.log")
		require.NoError(t, c.Batch([]*entry.Entry{e}))
	}
	queueEntry(t, converter, "Something routine", entry.Info)
	queueEntry(t, converter, "Something bad happened!", entry.Error)
	queueEntry(t, converter, "Some details...", entry.Debug)

	dir, err := os.Getwd()
	require.NoError(t, err)
	t.Logf("Working Directory: %s", dir)

	wg.Wait()

	require.Eventually(t, expectNLogs(sink, 3), 2*time.Second, 5*time.Millisecond,
		"expected %d but got %d logs",
		3, sink.LogRecordCount(),
	)
	// TODO: Figure out a nice way to assert each logs entry content.
	// require.Equal(t, expectedLogs, sink.AllLogs())
	require.NoError(t, rcvr.Shutdown(context.Background()))
}

func TestReadRotatingFiles(t *testing.T) {

	tests := []rotationTest{
		{
			name:             "CopyTruncateTimestamped",
			copyTruncate:     true,
			sequential:       false,
			enableThreadPool: false,
		},
		{
			name:             "CopyTruncateSequential",
			copyTruncate:     true,
			sequential:       true,
			enableThreadPool: false,
		},
		{
			name:             "CopyTruncateTimestampedThreadPool",
			copyTruncate:     true,
			sequential:       false,
			enableThreadPool: true,
		},
		{
			name:             "CopyTruncateSequentialThreadPool",
			copyTruncate:     true,
			sequential:       true,
			enableThreadPool: true,
		},
	}
	if runtime.GOOS != "windows" {
		// Windows has very poor support for moving active files, so rotation is less commonly used
		tests = append(tests, []rotationTest{
			{
				name:             "MoveCreateTimestampedThreadPool",
				copyTruncate:     false,
				sequential:       false,
				enableThreadPool: true,
			},
			{
				name:             "MoveCreateSequentialThreadPool",
				copyTruncate:     false,
				sequential:       true,
				enableThreadPool: true,
			},
			{
				name:             "MoveCreateTimestamped",
				copyTruncate:     false,
				sequential:       false,
				enableThreadPool: false,
			},
			{
				name:             "MoveCreateSequential",
				copyTruncate:     false,
				sequential:       true,
				enableThreadPool: false,
			},
		}...)
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.Run)
	}
}

type rotationTest struct {
	name             string
	copyTruncate     bool
	sequential       bool
	enableThreadPool bool
}

func (rt *rotationTest) Run(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	if rt.enableThreadPool {
		t.Cleanup(func() {
			require.NoError(t, featuregate.GlobalRegistry().Set("filelog.useThreadPool", false))
		})
		require.NoError(t, featuregate.GlobalRegistry().Set("filelog.useThreadPool", true))
	}
	f := NewFactory()
	sink := new(consumertest.LogsSink)

	cfg := rotationTestConfig(tempDir)

	// With a max of 100 logs per file and 1 backup file, rotation will occur
	// when more than 100 logs are written, and deletion when more than 200 are written.
	// Write 300 and validate that we got the all despite rotation and deletion.
	logger := newRotatingLogger(t, tempDir, 100, 1, rt.copyTruncate, rt.sequential)
	numLogs := 300

	// Build expected outputs
	expectedTimestamp, _ := time.ParseInLocation("2006-01-02", "2020-08-25", time.Local)
	converter := adapter.NewConverter(zap.NewNop())
	converter.Start()

	var wg sync.WaitGroup
	wg.Add(1)
	go consumeNLogsFromConverter(converter.OutChannel(), numLogs, &wg)

	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))

	for i := 0; i < numLogs; i++ {
		msg := fmt.Sprintf("This is a simple log line with the number %3d", i)

		// Build the expected set by converting entries to pdata Logs...
		e := entry.New()
		e.Timestamp = expectedTimestamp
		require.NoError(t, e.Set(entry.NewBodyField("msg"), msg))
		require.NoError(t, converter.Batch([]*entry.Entry{e}))

		// ... and write the logs lines to the actual file consumed by receiver.
		logger.Printf("2020-08-25 %s", msg)
		time.Sleep(time.Millisecond)
	}

	wg.Wait()
	require.Eventually(t, expectNLogs(sink, numLogs), 2*time.Second, 10*time.Millisecond,
		"expected %d but got %d logs",
		numLogs, sink.LogRecordCount(),
	)
	// TODO: Figure out a nice way to assert each logs entry content.
	// require.Equal(t, expectedLogs, sink.AllLogs())
	require.NoError(t, rcvr.Shutdown(context.Background()))
	converter.Stop()
}

func consumeNLogsFromConverter(ch <-chan plog.Logs, count int, wg *sync.WaitGroup) {
	defer wg.Done()

	n := 0
	for pLog := range ch {
		n += pLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len()

		if n == count {
			return
		}
	}
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

func expectNLogs(sink *consumertest.LogsSink, expected int) func() bool {
	return func() bool { return sink.LogRecordCount() == expected }
}

func testdataConfigYaml() *FileLogConfig {
	return &FileLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{
				{
					Builder: func() *regex.Config {
						cfg := regex.NewConfig()
						cfg.Regex = "^(?P<time>\\d{4}-\\d{2}-\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"
						sevField := entry.NewAttributeField("sev")
						sevCfg := helper.NewSeverityConfig()
						sevCfg.ParseFrom = &sevField
						cfg.SeverityConfig = &sevCfg
						timeField := entry.NewAttributeField("time")
						timeCfg := helper.NewTimeParser()
						timeCfg.Layout = "%Y-%m-%d"
						timeCfg.ParseFrom = &timeField
						cfg.TimeParser = &timeCfg
						return cfg
					}(),
				},
			},
			RetryOnFailure: consumerretry.Config{
				Enabled:         false,
				InitialInterval: 1 * time.Second,
				MaxInterval:     30 * time.Second,
				MaxElapsedTime:  5 * time.Minute,
			},
		},
		InputConfig: func() file.Config {
			c := file.NewConfig()
			c.Include = []string{"testdata/simple.log"}
			c.StartAt = "beginning"
			return *c
		}(),
	}
}

func rotationTestConfig(tempDir string) *FileLogConfig {
	return &FileLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators: []operator.Config{
				{
					Builder: func() *regex.Config {
						cfg := regex.NewConfig()
						cfg.Regex = "^(?P<ts>\\d{4}-\\d{2}-\\d{2}) (?P<msg>[^\n]+)"
						timeField := entry.NewAttributeField("ts")
						timeCfg := helper.NewTimeParser()
						timeCfg.Layout = "%Y-%m-%d"
						timeCfg.ParseFrom = &timeField
						cfg.TimeParser = &timeCfg
						return cfg
					}(),
				},
			},
		},
		InputConfig: func() file.Config {
			c := file.NewConfig()
			c.Include = []string{fmt.Sprintf("%s/*", tempDir)}
			c.StartAt = "beginning"
			c.PollInterval = 10 * time.Millisecond
			c.IncludeFileName = false
			return *c
		}(),
	}
}

// TestConsumeContract tests the contract between the filelog receiver and the next consumer with enabled retry.
func TestConsumeContract(t *testing.T) {
	tmpDir := t.TempDir()
	filePattern := "test-*.log"
	flg := &fileLogGenerator{t: t, tmpDir: tmpDir, filePattern: filePattern}

	cfg := createDefaultConfig()
	cfg.RetryOnFailure.Enabled = true
	cfg.RetryOnFailure.InitialInterval = 1 * time.Millisecond
	cfg.RetryOnFailure.MaxInterval = 10 * time.Millisecond
	cfg.InputConfig.Include = []string{filepath.Join(tmpDir, filePattern)}
	cfg.InputConfig.StartAt = "beginning"
	jsonParser := json.NewConfig()
	tsField := entry.NewAttributeField("ts")
	jsonParser.TimeParser = &helper.TimeParser{
		ParseFrom:  &tsField,
		Layout:     time.RFC3339,
		LayoutType: "gotime",
	}
	jsonParser.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
	logField := entry.NewAttributeField("log")
	jsonParser.BodyField = &logField
	cfg.Operators = []operator.Config{{Builder: jsonParser}}

	receivertest.CheckConsumeContract(receivertest.CheckConsumeContractParams{
		T:             t,
		Factory:       NewFactory(),
		DataType:      component.DataTypeLogs,
		Config:        cfg,
		Generator:     flg,
		GenerateCount: 10000,
	})
}

type fileLogGenerator struct {
	t           *testing.T
	tmpDir      string
	filePattern string
	tmpFile     *os.File
	sequenceNum int64
}

func (g *fileLogGenerator) Start() {
	tmpFile, err := os.CreateTemp(g.tmpDir, g.filePattern)
	require.NoError(g.t, err)
	g.tmpFile = tmpFile
}

func (g *fileLogGenerator) Stop() {
	require.NoError(g.t, g.tmpFile.Close())
	require.NoError(g.t, os.Remove(g.tmpFile.Name()))
}

func (g *fileLogGenerator) Generate() []receivertest.UniqueIDAttrVal {
	id := receivertest.UniqueIDAttrVal(fmt.Sprintf("%d", atomic.AddInt64(&g.sequenceNum, 1)))
	logLine := fmt.Sprintf(`{"ts": "%s", "log": "log-%s", "%s": "%s"}`, time.Now().Format(time.RFC3339), id,
		receivertest.UniqueIDAttrName, id)
	_, err := g.tmpFile.WriteString(logLine + "\n")
	require.NoError(g.t, err)
	return []receivertest.UniqueIDAttrVal{id}
}
