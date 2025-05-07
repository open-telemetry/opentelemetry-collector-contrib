// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var (
	s3PrefixKey    = "_sourceHost"
	prefixValue = "host"
	testLogs       = []byte(fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":[{"key":"_sourceCategory","value":{"stringValue":"logfile"}},{"key":"%s","value":{"stringValue":"%s"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1654257420681895000","body":{"stringValue":"2022-06-03 13:57:00.62739 +0200 CEST m=+14.018296742 log entry14"},"attributes":[{"key":"log.file.path_resolved","value":{"stringValue":"logwriter/data.log"}}],"traceId":"","spanId":""}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.6.1"}]}`, s3PrefixKey, prefixValue))
)

type TestWriter func(context.Context, []byte, pcommon.Map) error

func (testWriter TestWriter) Upload(ctx context.Context, buf []byte, attrs pcommon.Map) error {
	return testWriter(ctx, buf, attrs)
}

func getTestLogs(tb testing.TB) plog.Logs {
	logsMarshaler := plog.JSONUnmarshaler{}
	logs, err := logsMarshaler.UnmarshalLogs(testLogs)
	assert.NoError(tb, err, "Can't unmarshal testing logs data -> %s", err)
	return logs
}

func getLogExporter(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	testWriter := TestWriter(func(ctx context.Context, buf []byte, attrs pcommon.Map) error {
		assert.Equal(t, testLogs, buf)
		val, ok := attrs.Get(s3PrefixKey)
		assert.True(t, ok)
		assert.Equal(t, prefixValue, val.AsString())
		return nil
	})
	exporter := &s3Exporter{
		config:    createDefaultConfig().(*Config),
		uploader:  testWriter,
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLog(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporter(t)
	assert.NoError(t, exporter.ConsumeLogs(context.Background(), logs))
}
