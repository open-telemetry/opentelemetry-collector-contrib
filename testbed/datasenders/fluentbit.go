// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasenders

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type FluentBitFileLogWriter struct {
	testbed.DataSenderBase
	file        *os.File
	parsersFile *os.File
}

// Ensure FluentBitFileLogWriter implements LogDataSender
var _ testbed.LogDataSender = (*FluentBitFileLogWriter)(nil)

// NewFluentBitFileLogWriter creates a new data sender that will write log entries to a
// file, to be tailed by FluentBit and sent to the collector.
func NewFluentBitFileLogWriter(host string, port int) *FluentBitFileLogWriter {
	file, err := ioutil.TempFile("", "perf-logs.json")
	if err != nil {
		panic("failed to create temp file")
	}

	parsersFile, err := ioutil.TempFile("", "parsers.json")
	if err != nil {
		panic("failed to create temp file")
	}

	f := &FluentBitFileLogWriter{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: host,
		},
		file:        file,
		parsersFile: parsersFile,
	}
	f.setupParsers()
	return f
}

func (f *FluentBitFileLogWriter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *FluentBitFileLogWriter) Start() error {
	if _, err := exec.LookPath("fluent-bit"); err != nil {
		return err
	}

	return nil
}

func (f *FluentBitFileLogWriter) setupParsers() {
	_, err := f.parsersFile.Write([]byte(`
[PARSER]
    Name   json
    Format json
    Time_Key time
    Time_Format %d/%m/%Y:%H:%M:%S %z
`))
	if err != nil {
		panic("failed to write parsers")
	}

	f.parsersFile.Close()
}

func (f *FluentBitFileLogWriter) ConsumeLogs(_ context.Context, logs pdata.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).InstrumentationLibraryLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).InstrumentationLibraryLogs().At(j)
			for k := 0; k < ills.Logs().Len(); k++ {
				_, err := f.file.Write(append(f.convertLogToJSON(ills.Logs().At(k)), '\n'))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (f *FluentBitFileLogWriter) convertLogToJSON(lr pdata.LogRecord) []byte {
	rec := map[string]string{
		"time": time.Unix(0, int64(lr.Timestamp())).Format("02/01/2006:15:04:05Z"),
	}
	rec["log"] = lr.Body().StringVal()

	lr.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch v.Type() {
		case pdata.AttributeValueTypeString:
			rec[k] = v.StringVal()
		case pdata.AttributeValueTypeInt:
			rec[k] = strconv.FormatInt(v.IntVal(), 10)
		case pdata.AttributeValueTypeDouble:
			rec[k] = strconv.FormatFloat(v.DoubleVal(), 'f', -1, 64)
		case pdata.AttributeValueTypeBool:
			rec[k] = strconv.FormatBool(v.BoolVal())
		default:
			panic("missing case")
		}
		return true
	})
	b, err := json.Marshal(rec)
	if err != nil {
		panic("failed to write log: " + err.Error())
	}
	return b
}

func (f *FluentBitFileLogWriter) Flush() {
	_ = f.file.Sync()
}

func (f *FluentBitFileLogWriter) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  fluentforward:
    endpoint: "%s"`, f.GetEndpoint())
}

func (f *FluentBitFileLogWriter) Extensions() map[string]string {
	return map[string]string{
		"fluentbit": fmt.Sprintf(`
  fluentbit:
    executable_path: fluent-bit
    tcp_endpoint: "%s"
    config: |
      [SERVICE]
        parsers_file %s
      [INPUT]
        Name tail
        parser json
        path %s
`, f.GetEndpoint(), f.parsersFile.Name(), f.file.Name()),
	}
}

func (f *FluentBitFileLogWriter) ProtocolName() string {
	return "fluentforward"
}
