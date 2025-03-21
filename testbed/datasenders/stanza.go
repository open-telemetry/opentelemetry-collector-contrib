// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type FileLogWriter struct {
	file    *os.File
	retry   string
	storage string
}

// Ensure FileLogWriter implements LogDataSender.
var _ testbed.LogDataSender = (*FileLogWriter)(nil)

// NewFileLogWriter creates a new data sender that will write log entries to a
// file, to be tailed by FluentBit and sent to the collector.
func NewFileLogWriter(t *testing.T) *FileLogWriter {
	file, err := os.CreateTemp(t.TempDir(), "perf-logs.log")
	if err != nil {
		panic("failed to create temp file")
	}

	f := &FileLogWriter{
		file: file,
	}

	return f
}

func (f *FileLogWriter) WithRetry(retry string) *FileLogWriter {
	f.retry = retry
	return f
}

func (f *FileLogWriter) WithStorage(storage string) *FileLogWriter {
	f.storage = storage
	return f
}

func (f *FileLogWriter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *FileLogWriter) Start() error {
	return nil
}

func (f *FileLogWriter) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).ScopeLogs().At(j)
			for k := 0; k < ills.LogRecords().Len(); k++ {
				_, err := f.file.Write(append(f.convertLogToTextLine(ills.LogRecords().At(k)), '\n'))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *FileLogWriter) convertLogToTextLine(lr plog.LogRecord) []byte {
	sb := strings.Builder{}

	// Timestamp
	sb.WriteString(time.Unix(0, int64(lr.Timestamp())).Format("2006-01-02"))

	// Severity
	sb.WriteString(" ")
	sb.WriteString(lr.SeverityText())
	sb.WriteString(" ")

	if lr.Body().Type() == pcommon.ValueTypeStr {
		sb.WriteString(lr.Body().Str())
	}

	for k, v := range lr.Attributes().All() {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		switch v.Type() {
		case pcommon.ValueTypeStr:
			sb.WriteString(v.Str())
		case pcommon.ValueTypeInt:
			sb.WriteString(strconv.FormatInt(v.Int(), 10))
		case pcommon.ValueTypeDouble:
			sb.WriteString(strconv.FormatFloat(v.Double(), 'f', -1, 64))
		case pcommon.ValueTypeBool:
			sb.WriteString(strconv.FormatBool(v.Bool()))
		default:
			panic("missing case")
		}
	}

	return []byte(sb.String())
}

func (f *FileLogWriter) Flush() {
	_ = f.file.Sync()
}

func (f *FileLogWriter) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	// We are testing stanza receiver here.
	return fmt.Sprintf(`
  filelog:
    include: [ %s ]
    start_at: beginning
    operators:
      - type: regex_parser
        regex: '^(?P<time>\d{4}-\d{2}-\d{2}) (?P<sev>[A-Z0-9]*) (?P<msg>.*)$'
        timestamp:
          parse_from: attributes.time
          layout: '%%Y-%%m-%%d'
        severity:
          parse_from: attributes.sev
    %s
    %s
`, f.file.Name(), f.retry, f.storage)
}

func (f *FileLogWriter) ProtocolName() string {
	return "filelog"
}

func (f *FileLogWriter) GetEndpoint() net.Addr {
	return nil
}

func NewLocalFileStorageExtension(t *testing.T) map[string]string {
	tempDir := t.TempDir()

	return map[string]string{
		"file_storage": fmt.Sprintf(`
  file_storage:
    directory: %s
`, tempDir),
	}
}
