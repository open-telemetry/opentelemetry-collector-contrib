// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

type encodingExtension struct {
	extension ptrace.Unmarshaler
	suffix    string
}

type encodingExtensions []encodingExtension

type awss3TraceReceiver struct {
	s3Reader        *s3Reader
	consumer        consumer.Traces
	logger          *zap.Logger
	cancel          context.CancelFunc
	obsrecv         *receiverhelper.ObsReport
	encodingsConfig []Encoding
	extensions      encodingExtensions
}

func newAWSS3TraceReceiver(ctx context.Context, cfg *Config, traces consumer.Traces, settings receiver.Settings) (*awss3TraceReceiver, error) {
	reader, err := newS3Reader(ctx, cfg)
	if err != nil {
		return nil, err
	}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "s3",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	return &awss3TraceReceiver{
		s3Reader:        reader,
		consumer:        traces,
		logger:          settings.Logger,
		cancel:          nil,
		obsrecv:         obsrecv,
		encodingsConfig: cfg.Encodings,
	}, nil
}

func (r *awss3TraceReceiver) Start(_ context.Context, host component.Host) error {
	var err error
	r.extensions, err = newEncodingExtensions(r.encodingsConfig, host)
	if err != nil {
		return err
	}

	var ctx context.Context
	ctx, r.cancel = context.WithCancel(context.Background())
	go func() {
		_ = r.s3Reader.readAll(ctx, "traces", r.receiveBytes)
	}()
	return nil
}

func (r *awss3TraceReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

func (r *awss3TraceReceiver) receiveBytes(ctx context.Context, key string, data []byte) error {
	if data == nil {
		return nil
	}

	if strings.HasSuffix(key, ".gz") {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		key = strings.TrimSuffix(key, ".gz")
		data, err = io.ReadAll(reader)
		if err != nil {
			return err
		}
	}

	unmarshaler, format := r.extensions.findExtension(key)
	if unmarshaler == nil {
		if strings.HasSuffix(key, ".json") {
			unmarshaler = &ptrace.JSONUnmarshaler{}
			format = "otlp_json"
		}
		if strings.HasSuffix(key, ".binpb") {
			unmarshaler = &ptrace.ProtoUnmarshaler{}
			format = "otlp_proto"
		}
	}
	if unmarshaler == nil {
		r.logger.Warn("Unsupported file format", zap.String("key", key))
		return nil
	}
	traces, err := unmarshaler.UnmarshalTraces(data)
	if err != nil {
		return err
	}
	obsCtx := r.obsrecv.StartTracesOp(ctx)
	err = r.consumer.ConsumeTraces(ctx, traces)
	r.obsrecv.EndTracesOp(obsCtx, format, traces.SpanCount(), err)
	return err
}

func newEncodingExtensions(encodingsConfig []Encoding, host component.Host) (encodingExtensions, error) {
	encodings := make(encodingExtensions, 0)
	extensions := host.GetExtensions()
	for _, encoding := range encodingsConfig {
		if e, ok := extensions[encoding.Extension]; ok {
			if u, ok := e.(ptrace.Unmarshaler); ok {
				encodings = append(encodings, encodingExtension{extension: u, suffix: encoding.Suffix})
			}
		} else {
			return nil, fmt.Errorf("extension %q not found", encoding.Extension)
		}
	}
	return encodings, nil
}

func (encodings encodingExtensions) findExtension(key string) (ptrace.Unmarshaler, string) {
	for _, encoding := range encodings {
		if strings.HasSuffix(key, encoding.suffix) {
			return encoding.extension, encoding.suffix
		}
	}
	return nil, ""
}
