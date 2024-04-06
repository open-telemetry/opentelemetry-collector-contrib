// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"strings"
)

type awss3TraceReceiver struct {
	s3Reader *s3Reader
	consumer consumer.Traces
	logger   *zap.Logger
	cancel   context.CancelFunc
}

func newAWSS3TraceReceiver(cfg *Config, traces consumer.Traces, logger *zap.Logger) (*awss3TraceReceiver, error) {
	reader, err := newS3Reader(cfg)
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

func (r *awss3TraceReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)
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
	var unmarshaler ptrace.Unmarshaler
	if strings.HasSuffix(key, ".json") {
		unmarshaler = &ptrace.JSONUnmarshaler{}
	}
	if strings.HasSuffix(key, ".binpb") {
		unmarshaler = &ptrace.ProtoUnmarshaler{}
	}
	if unmarshaler != nil {
		traces, err := unmarshaler.UnmarshalTraces(data)
		if err != nil {
			return err
		}
		return r.consumer.ConsumeTraces(ctx, traces)
	}
	return nil
}
