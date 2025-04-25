// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/upload"
)

type s3Exporter struct {
	config     *Config
	signalType string
	uploader   upload.Manager
	logger     *zap.Logger
	marshaler  marshaler
}

func newS3Exporter(
	config *Config,
	signalType string,
	params exporter.Settings,
) *s3Exporter {
	s3Exporter := &s3Exporter{
		config:     config,
		signalType: signalType,
		logger:     params.Logger,
	}
	return s3Exporter
}

func (e *s3Exporter) getUploadOpts(res pcommon.Resource) *upload.UploadOptions {
	s3Prefix := ""
	if s3PrefixKey := e.config.ResourceAttrsToS3.S3Prefix; s3PrefixKey != "" {
		if value, ok := res.Attributes().Get(s3PrefixKey); ok {
			s3Prefix = value.AsString()
		}
	}
	uploadOpts := &upload.UploadOptions{
		OverridePrefix: s3Prefix,
	}
	return uploadOpts
}

func (e *s3Exporter) start(ctx context.Context, host component.Host) error {
	var m marshaler
	var err error
	if e.config.Encoding != nil {
		if m, err = newMarshalerFromEncoding(e.config.Encoding, e.config.EncodingFileExtension, host, e.logger); err != nil {
			return err
		}
	} else {
		if m, err = newMarshaler(e.config.MarshalerName, e.logger); err != nil {
			return fmt.Errorf("unknown marshaler %q", e.config.MarshalerName)
		}
	}

	e.marshaler = m

	up, err := newUploadManager(ctx, e.config, e.signalType, m.format())
	if err != nil {
		return err
	}
	e.uploader = up
	return nil
}

func (e *s3Exporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *s3Exporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	buf, err := e.marshaler.MarshalMetrics(md)
	if err != nil {
		return err
	}

	uploadOpts := e.getUploadOpts(md.ResourceMetrics().At(0).Resource())
	return e.uploader.Upload(ctx, buf, uploadOpts)
}

func (e *s3Exporter) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	buf, err := e.marshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}

	uploadOpts := e.getUploadOpts(logs.ResourceLogs().At(0).Resource())

	return e.uploader.Upload(ctx, buf, uploadOpts)
}

func (e *s3Exporter) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	buf, err := e.marshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}

	uploadOpts := e.getUploadOpts(traces.ResourceSpans().At(0).Resource())

	return e.uploader.Upload(ctx, buf, uploadOpts)
}
