// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type awss3TraceReceiver struct {
	s3Reader *s3Reader
	consumer consumer.Traces
	logger   *zap.Logger
	cancel   context.CancelFunc
}

func newAWSS3TraceReceiver(ctx context.Context, cfg *Config, traces consumer.Traces, logger *zap.Logger) (*awss3TraceReceiver, error) {
	reader, err := newS3Reader(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &awss3TraceReceiver{
		s3Reader: reader,
		consumer: traces,
		logger:   logger,
		cancel:   nil,
	}, nil
}

func (r *awss3TraceReceiver) Start(_ context.Context, _ component.Host) error {
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

	var unmarshaler ptrace.Unmarshaler
	if strings.HasSuffix(key, ".json") {
		unmarshaler = &ptrace.JSONUnmarshaler{}
	}
	if strings.HasSuffix(key, ".binpb") {
		unmarshaler = &ptrace.ProtoUnmarshaler{}
	}
	if unmarshaler == nil {
		r.logger.Warn("Unsupported file format", zap.String("key", key))
		return nil
	}
	traces, err := unmarshaler.UnmarshalTraces(data)
	if err != nil {
		return err
	}
	return r.consumer.ConsumeTraces(ctx, traces)
}
