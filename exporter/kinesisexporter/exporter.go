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

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumererror"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	jaegertranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	kinesis "github.com/signalfx/opencensus-go-exporter-kinesis"
	"go.uber.org/zap"
)

// Exporter implements an OpenTelemetry trace exporter that exports all spans to AWS Kinesis
type Exporter struct {
	kinesis *kinesis.Exporter
	logger  *zap.Logger
}

var _ (exporter.TraceExporter) = (*Exporter)(nil)

// Start tells the exporter to start. The exporter may prepare for exporting
// by connecting to the endpoint. Host parameter can be used for communicating
// with the host after Start() has already returned. If error is returned by
// Start() then the collector startup will be aborted.
func (e Exporter) Start(host exporter.Host) error {
	return nil
}

// Shutdown is invoked during exporter shutdown.
func (e Exporter) Shutdown() error {
	e.kinesis.Flush()
	return nil
}

// ConsumeTraceData receives a span batch and exports it to AWS Kinesis
func (e Exporter) ConsumeTraceData(c context.Context, td consumerdata.TraceData) error {
	pBatch, err := jaegertranslator.OCProtoToJaegerProto(td)
	if err != nil {
		e.logger.Error("error translating span batch", zap.Error(err))
		return consumererror.Permanent(err)
	}
	// TODO: Use a multi error type
	var exportErr error
	for _, span := range pBatch.GetSpans() {
		if span.Process == nil {
			span.Process = pBatch.Process
		}
		err := e.kinesis.ExportSpan(span)
		if err != nil {
			e.logger.Error("error exporting span to kinesis", zap.Error(err))
			exportErr = err
		}
	}
	return exportErr
}
