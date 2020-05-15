// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package sapmexporter exports trace data using Splunk's SAPM protocol.
package sapmexporter

import (
	"context"

	sapmclient "github.com/signalfx/sapm-proto/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/translator/trace/jaeger"
	"go.uber.org/zap"
)

// sapmExporter is a wrapper struct of SAPM exporter
type sapmExporter struct {
	client *sapmclient.Client
	logger *zap.Logger
}

func (se *sapmExporter) Shutdown(context.Context) error {
	se.client.Stop()
	return nil
}

func newSAPMTraceExporter(cfg *Config, params component.ExporterCreateParams) (component.TraceExporter, error) {
	err := cfg.validate()
	if err != nil {
		return nil, err
	}

	client, err := sapmclient.New(cfg.clientOptions()...)
	if err != nil {
		return nil, err
	}
	se := sapmExporter{
		client: client,
		logger: params.Logger,
	}
	return exporterhelper.NewTraceExporter(
		cfg,
		se.pushTraceData,
		exporterhelper.WithShutdown(se.Shutdown))
}

// pushTraceData exports traces in SAPM proto and returns number of dropped spans and error if export failed
func (se *sapmExporter) pushTraceData(ctx context.Context, td pdata.Traces) (droppedSpansCount int, err error) {
	batches, err := jaeger.InternalTracesToJaegerProto(td)
	if err != nil {
		return td.SpanCount(), consumererror.Permanent(err)
	}
	err = se.client.Export(ctx, batches)
	if err != nil {
		if sendErr, ok := err.(*sapmclient.ErrSend); ok {
			if sendErr.Permanent {
				return 0, consumererror.Permanent(sendErr)
			}
		}
		return td.SpanCount(), err
	}
	return 0, nil
}
