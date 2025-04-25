// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
)

var (
	s3PrefixKey    = "_sourceHost"
	overridePrefix = "host"
	testLogs       = []byte(fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":[{"key":"_sourceCategory","value":{"stringValue":"logfile"}},{"key":"%s","value":{"stringValue":"%s"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1654257420681895000","body":{"stringValue":"2022-06-03 13:57:00.62739 +0200 CEST m=+14.018296742 log entry14"},"attributes":[{"key":"log.file.path_resolved","value":{"stringValue":"logwriter/data.log"}}],"traceId":"","spanId":""}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.6.1"}]}`, s3PrefixKey, overridePrefix))
)

type TestWriter struct {
	t *testing.T
}

func (testWriter *TestWriter) Upload(_ context.Context, buf []byte, uploadOpts *upload.UploadOptions) error {
	assert.Equal(testWriter.t, testLogs, buf)
	assert.Equal(testWriter.t, &upload.UploadOptions{OverridePrefix: ""}, uploadOpts)
	return nil
}

func getTestLogs(tb testing.TB) plog.Logs {
	logsMarshaler := plog.JSONUnmarshaler{}
	logs, err := logsMarshaler.UnmarshalLogs(testLogs)
	assert.NoError(tb, err, "Can't unmarshal testing logs data -> %s", err)
	return logs
}

func getLogExporter(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	exporter := &s3Exporter{
		config:    createDefaultConfig().(*Config),
		uploader:  &TestWriter{t},
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

type TestWriterWithResourceAttrs struct {
	t *testing.T
}

func (testWriterWO *TestWriterWithResourceAttrs) Upload(_ context.Context, buf []byte, uploadOpts *upload.UploadOptions) error {
	assert.Equal(testWriterWO.t, testLogs, buf)
	assert.Equal(testWriterWO.t, &upload.UploadOptions{OverridePrefix: overridePrefix}, uploadOpts)
	return nil
}

func getLogExporterWithResourceAttrs(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	config := createDefaultConfig().(*Config)
	config.ResourceAttrsToS3.S3Prefix = s3PrefixKey
	exporter := &s3Exporter{
		config:    config,
		uploader:  &TestWriterWithResourceAttrs{t},
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLogWithResourceAttrs(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporterWithResourceAttrs(t)
	assert.NoError(t, exporter.ConsumeLogs(context.Background(), logs))
}
