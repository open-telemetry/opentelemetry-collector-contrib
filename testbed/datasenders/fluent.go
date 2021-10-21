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
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

const (
	fluentDatafileVar = "FLUENT_DATA_SENDER_DATA_FILE"
	fluentPortVar     = "FLUENT_DATA_SENDER_RECEIVER_PORT"
)

// FluentLogsForwarder forwards logs to fluent forwader
type FluentLogsForwarder struct {
	testbed.DataSenderBase
	fluentLogger *fluent.Fluent
	dataFile     *os.File
}

// Ensure FluentLogsForwarder implements LogDataSender.
var _ testbed.LogDataSender = (*FluentLogsForwarder)(nil)

func NewFluentLogsForwarder(t *testing.T, port int) *FluentLogsForwarder {
	var err error
	portOverride := os.Getenv(fluentPortVar)
	if portOverride != "" {
		port, err = strconv.Atoi(portOverride)
		require.NoError(t, err)
	}

	f := &FluentLogsForwarder{DataSenderBase: testbed.DataSenderBase{Port: port}}

	// When FLUENT_DATA_SENDER_DATA_FILE is set, the data sender, writes to a
	// file. This enables users to optionally run the e2e test against a real
	// fluentd/fluentbit agent rather than using the fluent writer the data sender
	// uses by default. In case, one is looking to point a real fluentd/fluentbit agent
	// to the e2e test, they can do so by configuring the fluent agent to read from the
	// file FLUENT_DATA_SENDER_DATA_FILE and forward data to FLUENT_DATA_SENDER_RECEIVER_PORT
	// on 127.0.0.1.
	if dataFileName := os.Getenv(fluentDatafileVar); dataFileName != "" {
		f.dataFile, err = os.OpenFile(dataFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		require.NoError(t, err)
	} else {
		logger, err := fluent.New(fluent.Config{FluentPort: port, Async: true})
		require.NoError(t, err)
		f.fluentLogger = logger
	}
	return f
}

func (f *FluentLogsForwarder) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *FluentLogsForwarder) Start() error {
	return nil
}

func (f *FluentLogsForwarder) Stop() error {
	return f.fluentLogger.Close()
}

func (f *FluentLogsForwarder) ConsumeLogs(_ context.Context, logs pdata.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		for j := 0; j < logs.ResourceLogs().At(i).InstrumentationLibraryLogs().Len(); j++ {
			ills := logs.ResourceLogs().At(i).InstrumentationLibraryLogs().At(j)
			for k := 0; k < ills.Logs().Len(); k++ {
				if f.dataFile == nil {
					if err := f.fluentLogger.Post("", f.convertLogToMap(ills.Logs().At(k))); err != nil {
						return err
					}
				} else {
					if _, err := f.dataFile.Write(append(f.convertLogToJSON(ills.Logs().At(k)), '\n')); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (f *FluentLogsForwarder) convertLogToMap(lr pdata.LogRecord) map[string]string {
	out := map[string]string{}

	if lr.Body().Type() == pdata.AttributeValueTypeString {
		out["log"] = lr.Body().StringVal()
	}

	lr.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch v.Type() {
		case pdata.AttributeValueTypeString:
			out[k] = v.StringVal()
		case pdata.AttributeValueTypeInt:
			out[k] = strconv.FormatInt(v.IntVal(), 10)
		case pdata.AttributeValueTypeDouble:
			out[k] = strconv.FormatFloat(v.DoubleVal(), 'f', -1, 64)
		case pdata.AttributeValueTypeBool:
			out[k] = strconv.FormatBool(v.BoolVal())
		default:
			panic("missing case")
		}
		return true
	})

	return out
}

func (f *FluentLogsForwarder) convertLogToJSON(lr pdata.LogRecord) []byte {
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

func (f *FluentLogsForwarder) Flush() {
	_ = f.dataFile.Sync()
}

func (f *FluentLogsForwarder) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  fluentforward:
    endpoint: localhost:%d`, f.Port)
}

func (f *FluentLogsForwarder) ProtocolName() string {
	return "fluentforward"
}
