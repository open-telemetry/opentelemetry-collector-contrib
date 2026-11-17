// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
)

var (
	s3PrefixKey    = "_sourceHost"
	s3BucketKey    = "_sourceBucket"
	overridePrefix = "host"
	overrideBucket = "my-bucket"
	testLogs       = fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":[{"key":"_sourceCategory","value":{"stringValue":"logfile"}},{"key":"%s","value":{"stringValue":"%s"}},{"key":"%s","value":{"stringValue":"%s"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"observedTimeUnixNano":"1654257420681895000","body":{"stringValue":"2022-06-03 13:57:00.62739 +0200 CEST m=+14.018296742 log entry14"},"attributes":[{"key":"log.file.path_resolved","value":{"stringValue":"logwriter/data.log"}}]}]}],"schemaUrl":"https://opentelemetry.io/schemas/1.6.1"}]}`, s3PrefixKey, overridePrefix, s3BucketKey, overrideBucket) //nolint:gocritic //sprintfQuotedString for JSON
)

type testWriter struct {
	t            *testing.T
	expectedOpts *upload.UploadOptions
}

func (testWriter *testWriter) Upload(_ context.Context, buf []byte, uploadOpts *upload.UploadOptions) error {
	assert.JSONEq(testWriter.t, testLogs, string(buf))
	assert.Equal(testWriter.t, testWriter.expectedOpts, uploadOpts)
	return nil
}

func getTestLogs(tb testing.TB) plog.Logs {
	logsMarshaler := plog.JSONUnmarshaler{}
	logs, err := logsMarshaler.UnmarshalLogs([]byte(testLogs))
	assert.NoError(tb, err, "Can't unmarshal testing the logs data -> %s", err)
	return logs
}

func getLogExporter(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	exporter := &s3Exporter{
		config:    createDefaultConfig().(*Config),
		uploader:  &testWriter{t: t, expectedOpts: &upload.UploadOptions{OverridePrefix: ""}},
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLog(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporter(t)
	assert.NoError(t, exporter.ConsumeLogs(t.Context(), logs))
}

func getLogExporterWithResourceAttrs(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	config := createDefaultConfig().(*Config)
	config.ResourceAttrsToS3.S3Prefix = s3PrefixKey
	exporter := &s3Exporter{
		config:    config,
		uploader:  &testWriter{t: t, expectedOpts: &upload.UploadOptions{OverridePrefix: overridePrefix}},
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLogWithResourceAttrs(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporterWithResourceAttrs(t)
	assert.NoError(t, exporter.ConsumeLogs(t.Context(), logs))
}

func getLogExporterWithBucketAndPrefixAttrs(t *testing.T) *s3Exporter {
	marshaler, _ := newMarshaler("otlp_json", zap.NewNop())
	config := createDefaultConfig().(*Config)
	config.ResourceAttrsToS3.S3Bucket = s3BucketKey
	config.ResourceAttrsToS3.S3Prefix = s3PrefixKey
	exporter := &s3Exporter{
		config:    config,
		uploader:  &testWriter{t: t, expectedOpts: &upload.UploadOptions{OverrideBucket: overrideBucket, OverridePrefix: overridePrefix}},
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}
	return exporter
}

func TestLogWithBucketAndPrefixAttrs(t *testing.T) {
	logs := getTestLogs(t)
	exporter := getLogExporterWithBucketAndPrefixAttrs(t)
	assert.NoError(t, exporter.ConsumeLogs(t.Context(), logs))
}

type recordingWriter struct {
	uploadedData []byte
	uploadOpts   *upload.UploadOptions
	uploadCalls  int
}

func (rw *recordingWriter) Upload(_ context.Context, buf []byte, uploadOpts *upload.UploadOptions) error {
	rw.uploadedData = buf
	rw.uploadOpts = uploadOpts
	rw.uploadCalls++
	return nil
}

func TestEmptyTelemetryDoesNotPanic(t *testing.T) {
	marshaler, err := newMarshaler("otlp_json", zap.NewNop())
	require.NoError(t, err)

	writer := &recordingWriter{}
	exporter := &s3Exporter{
		config:    createDefaultConfig().(*Config),
		uploader:  writer,
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}

	ctx := t.Context()

	// Empty logs
	assert.NotPanics(t, func() {
		err := exporter.ConsumeLogs(ctx, plog.NewLogs())
		assert.NoError(t, err)
	})
	assert.Equal(t, 0, writer.uploadCalls, "Upload should not be called for empty logs")

	// Empty metrics
	assert.NotPanics(t, func() {
		err := exporter.ConsumeMetrics(ctx, pmetric.NewMetrics())
		assert.NoError(t, err)
	})
	assert.Equal(t, 0, writer.uploadCalls, "Upload should not be called for empty metrics")

	// Empty traces
	assert.NotPanics(t, func() {
		err := exporter.ConsumeTraces(ctx, ptrace.NewTraces())
		assert.NoError(t, err)
	})
	assert.Equal(t, 0, writer.uploadCalls, "Upload should not be called for empty traces")
}

func TestConsumeMetrics(t *testing.T) {
	marshaler, err := newMarshaler("otlp_json", zap.NewNop())
	require.NoError(t, err)

	writer := &recordingWriter{}
	config := createDefaultConfig().(*Config)
	config.ResourceAttrsToS3.S3Bucket = s3BucketKey
	config.ResourceAttrsToS3.S3Prefix = s3PrefixKey

	exporter := &s3Exporter{
		config:    config,
		uploader:  writer,
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr(s3BucketKey, overrideBucket)
	rm.Resource().Attributes().PutStr(s3PrefixKey, overridePrefix)
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("system.cpu.load")
	metric.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.23)

	err = exporter.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)
	assert.Equal(t, 1, writer.uploadCalls)
	assert.Equal(t, overrideBucket, writer.uploadOpts.OverrideBucket)
	assert.Equal(t, overridePrefix, writer.uploadOpts.OverridePrefix)
	assert.Contains(t, string(writer.uploadedData), "system.cpu.load")
}

func TestConsumeTraces(t *testing.T) {
	marshaler, err := newMarshaler("otlp_json", zap.NewNop())
	require.NoError(t, err)

	writer := &recordingWriter{}
	config := createDefaultConfig().(*Config)
	config.ResourceAttrsToS3.S3Bucket = s3BucketKey
	config.ResourceAttrsToS3.S3Prefix = s3PrefixKey

	exporter := &s3Exporter{
		config:    config,
		uploader:  writer,
		logger:    zap.NewNop(),
		marshaler: marshaler,
	}

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(s3BucketKey, overrideBucket)
	rs.Resource().Attributes().PutStr(s3PrefixKey, overridePrefix)
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("GET /api/v1/resource")

	err = exporter.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)
	assert.Equal(t, 1, writer.uploadCalls)
	assert.Equal(t, overrideBucket, writer.uploadOpts.OverrideBucket)
	assert.Equal(t, overridePrefix, writer.uploadOpts.OverridePrefix)
	assert.Contains(t, string(writer.uploadedData), "GET /api/v1/resource")
}
