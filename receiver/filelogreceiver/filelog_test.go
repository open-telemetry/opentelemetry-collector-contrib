// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/jsonparser"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/metadata"
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

	sub, err := cm.Sub(component.MustNewID("filelog").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))
	assert.Equal(t, testdataConfigYaml(), cfg)
}

func TestCreateWithInvalidInputConfig(t *testing.T) {
	t.Parallel()

	cfg := testdataConfigYaml()
	cfg.InputConfig.StartAt = "middle"

	_, err := NewFactory().CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
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

	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	expectedLogs := []plog.Logs{}
	// Build the expected set by using adapter.Converter to translate entries
	// to pdata Logs.
	entries := []*entry.Entry{}
	queueEntry := func(msg string, severity entry.Severity) {
		e := entry.New()
		e.Timestamp = expectedTimestamp
		e.Body = fmt.Sprintf("2020-08-25 %s %s", severity.String(), msg)
		e.Severity = severity
		e.AddAttribute("log.file.name", "simple.log")
		e.AddAttribute("time", "2020-08-25")
		e.AddAttribute("sev", severity.String())
		e.AddAttribute("msg", msg)
		entries = append(entries, e)
	}
	queueEntry("Something routine", entry.Info)
	queueEntry("Something bad happened!", entry.Error)
	queueEntry("Some details...", entry.Debug)

	expectedLogs = append(expectedLogs, adapter.ConvertEntries(entries))

	dir, err := os.Getwd()
	require.NoError(t, err)
	t.Logf("Working Directory: %s", dir)

	require.Eventually(t, expectNLogs(sink, 3), 2*time.Second, 5*time.Millisecond,
		"expected %d but got %d logs",
		3, sink.LogRecordCount(),
	)

	for i, expectedLog := range expectedLogs {
		require.NoError(t,
			plogtest.CompareLogs(
				expectedLog,
				sink.AllLogs()[i],
				plogtest.IgnoreObservedTimestamp(),
				plogtest.IgnoreTimestamp(),
			),
		)
	}
	require.NoError(t, rcvr.Shutdown(t.Context()))
}

func TestReadRotatingFiles(t *testing.T) {
	tests := []rotationTest{
		{
			name:         "CopyTruncate",
			copyTruncate: true,
		},
	}
	if runtime.GOOS != "windows" {
		// Windows has very poor support for moving active files, so rotation is less commonly used
		tests = append(tests, []rotationTest{
			{
				name:         "MoveCreate",
				copyTruncate: false,
			},
		}...)
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
	sink := new(consumertest.LogsSink)

	tempDir := t.TempDir()
	cfg := rotationTestConfig(tempDir)

	// With a max of 100 logs per file and 1 backup file, rotation will occur
	// when more than 100 logs are written, and deletion when more than 200 are written.
	// Write 300 and validate that we got the all despite rotation and deletion.
	maxLinesPerFile := 100
	numLogs := 300
	fileName := filepath.Join(tempDir, "test.log")
	backupFileName := filepath.Join(tempDir, "test-backup.log")

	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0o600)
	defer func() {
		require.NoError(t, file.Close())
	}()
	require.NoError(t, err)

	for i := range numLogs {
		if (i+1)%maxLinesPerFile == 0 {
			if rt.copyTruncate {
				// Recreate the backup file
				// if backupFileName exists
				require.Eventually(t, func() bool {
					// On Windows you can't remove a file if it still has some handle opened to it. So remove the file
					// in a loop until any async operation on it is done.
					removeErr := os.Remove(backupFileName)
					return errors.Is(removeErr, os.ErrNotExist)
				}, 5*time.Second, 100*time.Millisecond)

				backupFile, openErr := os.OpenFile(backupFileName, os.O_CREATE|os.O_RDWR, 0o600)
				require.NoError(t, openErr)

				// Copy the current file to the backup file
				require.NoError(t, file.Sync())
				_, err = file.Seek(0, 0)
				require.NoError(t, err)
				_, err = io.Copy(backupFile, file)
				require.NoError(t, err)
				require.NoError(t, backupFile.Close())

				// Truncate the original file
				require.NoError(t, file.Truncate(0))
				_, err = file.Seek(0, 0)
				require.NoError(t, err)
			} else {
				require.NoError(t, file.Close())
				require.NoError(t, os.Rename(fileName, backupFileName))
				file, err = os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0o600)
				require.NoError(t, err)
			}
		}

		msg := fmt.Sprintf("This is a simple log line with the number %3d", i)

		// ... and write the logs lines to the actual file consumed by receiver.
		_, err := fmt.Fprintf(file, "2020-08-25 %s\n", msg)
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
	}

	require.Eventually(t, expectNLogs(sink, numLogs), 2*time.Second, 10*time.Millisecond,
		"expected %d but got %d logs",
		numLogs, sink.LogRecordCount(),
	)

	// TODO: Figure out a nice way to assert each logs entry content.
	// require.Equal(t, expectedLogs, sink.AllLogs())
	require.NoError(t, rcvr.Shutdown(t.Context()))
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
						// The Validate method modifies private fields on the helper.TimeParser
						// object that need to be set for later comparison.
						// If validation fails, the test will fail, so discard the error.
						_ = timeCfg.Validate()
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
			c.Include = []string{tempDir + "/*"}
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
	jsonParser := jsonparser.NewConfig()
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
		Signal:        pipeline.SignalLogs,
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
	seq := atomic.AddInt64(&g.sequenceNum, 1)
	id := receivertest.UniqueIDAttrVal(strconv.FormatInt(seq, 10))
	logLine := fmt.Sprintf(`{"ts": "%s", "log": "log-%s", "%s": "%s"}`, time.Now().Format(time.RFC3339), id, //nolint:gocritic //sprintfQuotedString for JSON
		receivertest.UniqueIDAttrName, id)
	_, err := g.tmpFile.WriteString(logLine + "\n")
	require.NoError(g.t, err)
	// Sync periodically to ensure data is flushed to disk for the file reader.
	// This is especially important on Windows where writes may not be visible
	// to other file handles until flushed.
	if seq%100 == 0 {
		require.NoError(g.t, g.tmpFile.Sync())
	}
	return []receivertest.UniqueIDAttrVal{id}
}

// TestMultilineWithUTF16Encoding tests that multiline patterns work correctly with UTF-16LE encoding.
// This is a regression test for https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/39011
func TestMultilineWithUTF16Encoding(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	fileName := filepath.Join(tempDir, "auditlog.txt")

	// Create a UTF-16LE encoded file with fixed-length records (no newlines)
	// Each record is 20 characters: "LOGSTART_01_content_"
	records := []string{
		"LOGSTART_01_content_",
		"LOGSTART_02_content_",
		"LOGSTART_03_content_",
		"LOGSTART_04_content_",
		"LOGSTART_05_content_",
	}

	// Write records in UTF-16LE encoding
	testFile, err := os.Create(fileName)
	require.NoError(t, err)

	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	utf16Writer := transform.NewWriter(testFile, encoder)

	for _, record := range records {
		_, err = utf16Writer.Write([]byte(record))
		require.NoError(t, err)
	}
	require.NoError(t, utf16Writer.Close())
	require.NoError(t, testFile.Close())

	// Configure filelog receiver with UTF-16LE encoding and multiline
	cfg := &FileLogConfig{
		BaseConfig: adapter.BaseConfig{},
		InputConfig: func() file.Config {
			c := file.NewConfig()
			c.Include = []string{fileName}
			c.StartAt = "beginning"
			c.PollInterval = 10 * time.Millisecond
			c.Encoding = "utf-16le"
			c.SplitConfig = split.Config{
				LineStartPattern: `LOGSTART_\d+_`,
			}
			c.FlushPeriod = 100 * time.Millisecond
			return *c
		}(),
	}

	f := NewFactory()
	sink := new(consumertest.LogsSink)

	rcvr, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))

	// We expect 5 log records since we have 5 records in the file
	// The last record will be flushed after the flush period
	require.Eventually(t, expectNLogs(sink, 5), 5*time.Second, 50*time.Millisecond,
		"expected %d but got %d logs",
		5, sink.LogRecordCount(),
	)

	// Verify each record was parsed correctly
	allLogs := sink.AllLogs()
	recordBodies := make([]string, 0, 5)
	for _, logs := range allLogs {
		for i := range logs.ResourceLogs().Len() {
			rl := logs.ResourceLogs().At(i)
			for j := range rl.ScopeLogs().Len() {
				sl := rl.ScopeLogs().At(j)
				for k := range sl.LogRecords().Len() {
					lr := sl.LogRecords().At(k)
					recordBodies = append(recordBodies, lr.Body().AsString())
				}
			}
		}
	}

	require.Len(t, recordBodies, 5, "expected 5 log records")
	for i, body := range recordBodies {
		expectedPrefix := fmt.Sprintf("LOGSTART_%02d_", i+1)
		assert.Contains(t, body, expectedPrefix, "record %d should contain expected prefix", i)
	}

	require.NoError(t, rcvr.Shutdown(t.Context()))
}
