// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasenders

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/observiq/nanojack"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/testbed/testbed"
)

// StanzaLogSender writes logs to a file for stanza to read
type StanzaLogSender struct {
	port   int
	path   string
	logger *nanojack.Logger
}

// Ensure StanzaLogSender implements LogDataSender.
var _ testbed.LogDataSender = (*StanzaLogSender)(nil)

// NewStanzaLogSender returns a new sender that writes logs to files in the configured directory
func NewStanzaLogSender(dir string, maxLines, maxBackups, port int) (*StanzaLogSender, error) {
	rotator := nanojack.Logger{
		Filename:   filepath.Join(dir, "out.log"),
		MaxLines:   maxLines,
		MaxBackups: maxBackups,
	}

	f := &StanzaLogSender{
		logger: &rotator,
		port:   port,
		path:   filepath.Join(dir, "*"),
	}
	return f, nil
}

// Start the sender.
func (ls *StanzaLogSender) Start() error {
	return nil
}

// SendLogs writes the given logs to file in JSON format
func (ls *StanzaLogSender) SendLogs(logs pdata.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).InstrumentationLibraryLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).InstrumentationLibraryLogs().At(j)
			for k := 0; k < ills.Logs().Len(); k++ {
				_, err := ls.logger.Write(append(ls.convertLogToJSON(ills.Logs().At(k)), '\n'))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// convertLogToJSON converts a log record to JSON. Taken from the Fluent Bit tests
func (ls *StanzaLogSender) convertLogToJSON(lr pdata.LogRecord) []byte {
	rec := map[string]string{
		"time": time.Unix(0, int64(lr.Timestamp())).Format("02/01/2006:15:04:05Z"),
	}
	if !lr.Body().IsNil() {
		rec["log"] = lr.Body().StringVal()
	}

	lr.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		switch v.Type() {
		case pdata.AttributeValueSTRING:
			rec[k] = v.StringVal()
		case pdata.AttributeValueINT:
			rec[k] = strconv.FormatInt(v.IntVal(), 10)
		case pdata.AttributeValueDOUBLE:
			rec[k] = strconv.FormatFloat(v.DoubleVal(), 'f', -1, 64)
		case pdata.AttributeValueBOOL:
			rec[k] = strconv.FormatBool(v.BoolVal())
		default:
			panic("missing case")
		}
	})
	b, err := json.Marshal(rec)
	if err != nil {
		panic("failed to write log: " + err.Error())
	}
	return b
}

// Flush flushes the sender
func (ls *StanzaLogSender) Flush() {}

// GetCollectorPort returns the port that the sender is listening on.
// For the StanzaLogSender, this is a fake port since stanza does not listen.
// A listener should be set up outside the log sender for this to work correctly.
func (ls *StanzaLogSender) GetCollectorPort() int {
	return ls.port
}

// GenConfigYAMLStr returns exporter config for the agent.
func (ls *StanzaLogSender) GenConfigYAMLStr() string {
	// Note that this generates an exporter config for agent.
	return fmt.Sprintf(`
  stanza:
    pipeline:
      - type: file_input
        include: ["%s"]
        start_at: beginning

      - type: json_parser
        timestamp:
          parse_from: time
          layout_type: gotime
          layout: '02/01/2006:15:04:05Z'`, ls.path)
}

func (ls *StanzaLogSender) Processors() map[string]string {
	return map[string]string{
		"batch": `
  batch:
`,
	}
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (ls *StanzaLogSender) ProtocolName() string {
	return "stanza"
}
