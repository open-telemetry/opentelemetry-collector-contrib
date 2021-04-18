// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
)

// pulsarTracesProducer produce trace messages to Pulsar.
type pulsarTracesProducer struct {
}

func (e *pulsarTracesProducer) tracesDataPusher(ctx context.Context, d pdata.Traces) error {
	// TODO implement
	return nil
}

// pulsarMetricsProducer produce metric messages to Pulsar.
type pulsarMetricsProducer struct {
}

func (e *pulsarMetricsProducer) metricsDataPusher(ctx context.Context, d pdata.Metrics) error {
	// TODO implement
	return nil
}

// pulsarLogsProducer produce log messages to Pulsar.
type pulsarLogsProducer struct {
}

func (e *pulsarLogsProducer) logsDataPusher(ctx context.Context, d pdata.Logs) error {
	// TODO implement
	return nil
}

// newTracesExporter creates Pulsar exporter.
func newTracesExporter(config Config, params component.ExporterCreateParams, marshallers map[string]TracesMarshaller) (*pulsarTracesProducer, error) {
	// TODO implement
	return nil, nil
}

func newMetricsExporter(config Config, params component.ExporterCreateParams, marshallers map[string]MetricsMarshaller) (*pulsarMetricsProducer, error) {
	// TODO implement
	return nil, nil

}

// newLogsExporter creates Pulsar exporter.
func newLogsExporter(config Config, params component.ExporterCreateParams, marshallers map[string]LogsMarshaller) (*pulsarLogsProducer, error) {
	// TODO implement
	return nil, nil
}
