// Copyright 2019 OpenTelemetry Authors
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

package kinesisexporter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	jaegertranslator "go.opentelemetry.io/collector/translator/trace/jaeger"
	"go.uber.org/zap"
)

const (
	errInvalidContext = "invalid context"
)

// Exporter implements an OpenTelemetry exporter that pushes OpenTelemetry data to AWS Kinesis
type Exporter struct {
	producer producer
	logger   *zap.Logger
}

// newExporter creates a new exporter with the passed in configurations.
// It starts the AWS session and setups the relevant connections.
func newExporter(c *Config, logger *zap.Logger) (*Exporter, error) {
	pr, err := newKinesisProducer(c, logger)
	if err != nil {
		return nil, err
	}

	return &Exporter{producer: pr, logger: logger}, nil
}

// Start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after start() has already returned. If error is returned by
// start() then the collector startup will be aborted.
func (e *Exporter) Start(ctx context.Context, _ component.Host) error {
	if ctx == nil || ctx.Err() != nil {
		return fmt.Errorf(errInvalidContext)
	}

	e.producer.start()
	return nil
}

// Shutdown is invoked during exporter shutdown
func (e *Exporter) Shutdown(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return fmt.Errorf(errInvalidContext)
	}

	e.producer.stop()
	return nil
}

func (e *Exporter) pushTraces(ctx context.Context, td pdata.Traces) (int, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, fmt.Errorf(errInvalidContext)
	}

	pBatches, err := jaegertranslator.InternalTracesToJaegerProto(td)
	if err != nil {
		e.logger.Error("error translating span batch", zap.Error(err))
		return td.SpanCount(), consumererror.Permanent(err)
	}

	// TODO: Use a multi error type
	var exportErr error
	for _, pBatch := range pBatches {
		for _, span := range pBatch.GetSpans() {
			if span.Process == nil {
				span.Process = pBatch.Process
			}

			spanBytes, err := span.Marshal()
			if err != nil {
				e.logger.Error("error marshalling span to bytes", zap.Error(err))
				exportErr = err
			}

			if err = e.producer.put(spanBytes, span.SpanID.String()); err != nil {
				e.logger.Error("error exporting span to kinesis", zap.Error(err))
				exportErr = err
			}
		}
	}

	return 0, exportErr
}
