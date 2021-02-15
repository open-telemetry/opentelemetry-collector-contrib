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

package datasenders

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/testbed/testbed"
)

// TODO: Extract common bits from FileLogWriter and NewFluentBitFileLogWriter
// and generalize as FileLogWriter.

type FileLogWriter struct {
	testbed.DataSenderBase
	file *os.File
}

// Ensure FileLogWriter implements LogDataSender.
var _ testbed.LogDataSender = (*FileLogWriter)(nil)

// NewFileLogWriter creates a new data sender that will write log entries to a
// file, to be tailed by FluentBit and sent to the collector.
func NewFileLogWriter() *FileLogWriter {
	file, err := ioutil.TempFile("", "perf-logs.log")
	if err != nil {
		panic("failed to create temp file")
	}

	f := &FileLogWriter{
		file: file,
	}

	return f
}

func (f *FileLogWriter) Start() error {
	return nil
}

func (f *FileLogWriter) ConsumeLogs(_ context.Context, logs pdata.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).InstrumentationLibraryLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).InstrumentationLibraryLogs().At(j)
			for k := 0; k < ills.Logs().Len(); k++ {
				_, err := f.file.Write(append(f.convertLogToTextLine(ills.Logs().At(k)), '\n'))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *FileLogWriter) convertLogToTextLine(lr pdata.LogRecord) []byte {
	sb := strings.Builder{}

	// Timestamp
	sb.WriteString(time.Unix(0, int64(lr.Timestamp())).Format("2006-01-02"))

	// Severity
	sb.WriteString(" ")
	sb.WriteString(lr.SeverityText())
	sb.WriteString(" ")

	if lr.Body().Type() == pdata.AttributeValueSTRING {
		sb.WriteString(lr.Body().StringVal())
	}

	lr.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		switch v.Type() {
		case pdata.AttributeValueSTRING:
			sb.WriteString(v.StringVal())
		case pdata.AttributeValueINT:
			sb.WriteString(strconv.FormatInt(v.IntVal(), 10))
		case pdata.AttributeValueDOUBLE:
			sb.WriteString(strconv.FormatFloat(v.DoubleVal(), 'f', -1, 64))
		case pdata.AttributeValueBOOL:
			sb.WriteString(strconv.FormatBool(v.BoolVal()))
		default:
			panic("missing case")
		}
	})

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
          parse_from: time
          layout: '%%Y-%%m-%%d'
        severity:
          parse_from: sev
`, f.file.Name())
}

func (f *FileLogWriter) ProtocolName() string {
	return "filelog"
}

func (f *FileLogWriter) GetEndpoint() string {
	return ""
}
