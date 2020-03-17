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

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumererror"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
	"github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	sapmclient "github.com/signalfx/sapm-proto/client"
	"go.uber.org/zap"
)

// sapmExporter is a wrapper struct of SAPM exporter
type sapmExporter struct {
	client *sapmclient.Client
	logger *zap.Logger
}

func (se *sapmExporter) Shutdown() error {
	se.client.Stop()
	return nil
}

func newSAPMTraceExporter(cfg *Config, logger *zap.Logger) (exporter.TraceExporter, error) {
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
		logger: logger,
	}
	return exporterhelper.NewTraceExporter(
		cfg,
		se.pushTraceData,
		exporterhelper.WithShutdown(se.Shutdown))
}

func (se *sapmExporter) pushTraceData(ctx context.Context, td consumerdata.TraceData) (int, error) {
	jBatch, err := jaeger.OCProtoToJaegerProto(td)
	if err != nil {
		return 0, consumererror.Permanent(err)
	}
	err = se.client.Export(ctx, jBatch)
	if err != nil {
		if sendErr, ok := err.(*sapmclient.ErrSend); ok {
			if sendErr.Permanent {
				return 0, consumererror.Permanent(sendErr)
			}
		}
		return 0, err
	}
	return len(td.Spans), nil
}
