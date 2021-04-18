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
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr                        = "pulsar"
	defaultTracesTopic             = "otlp_traces"
	defaultMetricsTopic            = "otlp_metrics"
	defaultLogsTopic               = "otlp_logs"
	defaultEncoding                = "otlp_proto"
	defaultConnectTimeout          = time.Second * 5
	defaultSendTimeout             = time.Second * 10
	defaultMaxPendingMessages      = 10000
	defaultCompressType            = pulsar.NoCompression
	defaultCompressLevel           = pulsar.Default
	defaultBatchingMaxPublishDelay = time.Millisecond * 10
	defaultBatchingMaxMessages     = 1000
	defaultBatchingMaxSize         = 1 * 1024 * 1024 // 1MB
	defaultURL                     = "pulsar://localhost:6650"
)

// FactoryOption applies changes to pulsarExporterFactory.
type FactoryOption func(factory *pulsarExporterFactory)

// WithAddTracesMarshallers adds tracesMarshallers.
func WithAddTracesMarshallers(encodingMarshaller map[string]TracesMarshaller) FactoryOption {
	return func(factory *pulsarExporterFactory) {
		for encoding, marshaller := range encodingMarshaller {
			factory.tracesMarshallers[encoding] = marshaller
		}
	}
}

// WithAddMetricsMarshallers adds metricsMarshallers.
func WithAddMetricsMarshallers(encodingMarshaller map[string]MetricsMarshaller) FactoryOption {
	return func(factory *pulsarExporterFactory) {
		for encoding, marshaller := range encodingMarshaller {
			factory.metricsMarshallers[encoding] = marshaller
		}
	}
}

// WithAddLogsMarshallers adds logsMarshallers.
func WithAddLogsMarshallers(encodingMarshaller map[string]LogsMarshaller) FactoryOption {
	return func(factory *pulsarExporterFactory) {
		for encoding, marshaller := range encodingMarshaller {
			factory.logsMarshallers[encoding] = marshaller
		}
	}
}

// NewFactory creates Pulsar exporter factory.
func NewFactory(options ...FactoryOption) component.ExporterFactory {
	f := &pulsarExporterFactory{
		tracesMarshallers:  tracesMarshallers(),
		metricsMarshallers: metricsMarshallers(),
		logsMarshallers:    logsMarshallers(),
	}
	for _, o := range options {
		o(f)
	}
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithTraces(f.createTracesExporter),
		exporterhelper.WithMetrics(f.createMetricsExporter),
		exporterhelper.WithLogs(f.createLogsExporter))
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		TimeoutSettings:  exporterhelper.DefaultTimeoutSettings(),
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),
		URL:              defaultURL,
		// using an empty topic to track when it has not been set by user, default is based on traces or metrics.
		Topic:                   "",
		Encoding:                defaultEncoding,
		ConnectionTimeout:       defaultConnectTimeout,
		SendTimeout:             defaultSendTimeout,
		MaxPendingMessages:      defaultMaxPendingMessages,
		CompressionType:         int(defaultCompressType),
		CompressionLevel:        int(defaultCompressLevel),
		BatchingMaxPublishDelay: defaultBatchingMaxPublishDelay,
		BatchingMaxMessages:     defaultBatchingMaxMessages,
		BatchingMaxSize:         defaultBatchingMaxSize,
	}
}

type pulsarExporterFactory struct {
	tracesMarshallers  map[string]TracesMarshaller
	metricsMarshallers map[string]MetricsMarshaller
	logsMarshallers    map[string]LogsMarshaller
}

func (f *pulsarExporterFactory) createTracesExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg config.Exporter,
) (component.TracesExporter, error) {
	oCfg := cfg.(*Config)
	if oCfg.Topic == "" {
		oCfg.Topic = defaultTracesTopic
	}
	exp, err := newTracesExporter(*oCfg, params, f.tracesMarshallers)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTracesExporter(
		cfg,
		params.Logger,
		exp.tracesDataPusher)
}

func (f *pulsarExporterFactory) createMetricsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg config.Exporter,
) (component.MetricsExporter, error) {
	oCfg := cfg.(*Config)
	if oCfg.Topic == "" {
		oCfg.Topic = defaultMetricsTopic
	}
	exp, err := newMetricsExporter(*oCfg, params, f.metricsMarshallers)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		cfg,
		params.Logger,
		exp.metricsDataPusher)
}

func (f *pulsarExporterFactory) createLogsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg config.Exporter,
) (component.LogsExporter, error) {
	oCfg := cfg.(*Config)
	if oCfg.Topic == "" {
		oCfg.Topic = defaultLogsTopic
	}
	exp, err := newLogsExporter(*oCfg, params, f.logsMarshallers)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(
		cfg,
		params.Logger,
		exp.logsDataPusher)
}
