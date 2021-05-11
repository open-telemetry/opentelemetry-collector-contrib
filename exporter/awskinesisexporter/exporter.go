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

package awskinesisexporter

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// exporter implements an OpenTelemetry exporter that pushes OpenTelemetry data to AWS Kinesis
type exporter struct {
	producer   producer
	logger     *zap.Logger
	marshaller Marshaller
}

// Ensuring that the exporter meets the interface at compile time
var (
	_ component.TracesExporter  = (*exporter)(nil)
	_ component.MetricsExporter = (*exporter)(nil)
)

// newExporter creates a new exporter with the passed in configurations.
// It starts the AWS session and setups the relevant connections.
func newExporter(c *Config, logger *zap.Logger) (*exporter, error) {
	// Get marshaller based on config
	marshaller := defaultMarshallers()[c.Encoding]
	if marshaller == nil {
		return nil, errors.New("unrecognized encoding")
	}

	pr, err := newKinesisProducer(c, logger)
	if err != nil {
		return nil, err
	}

	return &exporter{producer: pr, marshaller: marshaller, logger: logger}, nil
}

// Start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after start() has already returned. If error is returned by
// start() then the collector startup will be aborted.
func (e *exporter) Start(ctx context.Context, _ component.Host) error {
	if ctx == nil || ctx.Err() != nil {
		return errors.New("invalid context provided")
	}

	e.producer.start()
	return nil
}

// Shutdown is invoked during exporter shutdown.
func (e *exporter) Shutdown(_ context.Context) error {
	e.producer.stop()
	return nil
}

func (e *exporter) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	if ctx == nil || ctx.Err() != nil {
		return errors.New("invalid context provided")
	}

	pBatches, err := e.marshaller.MarshalTraces(td)
	if err != nil {
		consumererror.Permanent(err)
	}

	return e.producer.put(pBatches, uuid.New().String())
}

func (e *exporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	if ctx == nil || ctx.Err() != nil {
		return errors.New("invalid context provided")
	}

	pBatches, err := e.marshaller.MarshalMetrics(md)
	if err != nil {
		e.logger.Error("error translating metrics batch", zap.Error(err))
		return consumererror.Permanent(err)
	}

	return e.producer.put(pBatches, uuid.New().String())
}
