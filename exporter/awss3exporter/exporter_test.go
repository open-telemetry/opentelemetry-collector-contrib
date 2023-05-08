// Copyright The OpenTelemetry Authors
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

package awss3exporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var testLogs = []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"_sourceCategory","value":{"stringValue":"logfile"}},{"key":"_sourceHost","value":{"stringValue":"host"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1654257420681895000","body":{"stringValue":"2022-06-03 13:57:00.62739 +0200 CEST m=+14.018296742 log entry14"},"attributes":[{"key":"log.file.path_resolved","value":{"stringValue":"logwriter/data.log"}}],"traceId":"","spanId":""}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.6.1"}]}`)

type TestWriter struct {
	t *testing.T
}

func (testWriter *TestWriter) writeBuffer(ctx context.Context, buf []byte, config *Config, metadata string, format string) error {
	assert.Equal(testWriter.t, testLogs, buf)
	return nil
}

func getTestLogs(tb testing.TB) plog.Logs {
	logsMarshaler := plog.JSONUnmarshaler{}
	logs, err := logsMarshaler.UnmarshalLogs(testLogs)
	assert.NoError(tb, err, "Can't unmarshal testing logs data -> %s", err)
	return logs
}

func getLogExporter(t *testing.T) *s3Exporter {
	marshaler, _ := NewMarshaler("otlp_json", zap.NewNop())
	exporter := &s3Exporter{
		config:     createDefaultConfig().(*Config),
		dataWriter: &TestWriter{t},
		logger:     zap.NewNop(),
		marshaler:  marshaler,
	}
	return exporter
}

func TestLog(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporter(t)
	assert.NoError(t, exporter.ConsumeLogs(context.Background(), logs))
}
