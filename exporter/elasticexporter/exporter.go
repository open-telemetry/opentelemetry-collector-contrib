// Copyright 2020, OpenTelemetry Authors
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

// Package elasticexporter contains an opentelemetry-collector exporter
// for Elastic APM.
package elasticexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter"

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"go.elastic.co/apm/transport"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
)

func newElasticTracesExporter(
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.TracesExporter, error) {
	exporter, err := newElasticExporter(cfg.(*Config), set.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elastic APM trace exporter: %w", err)
	}
	return exporterhelper.NewTracesExporter(cfg, set, func(ctx context.Context, traces ptrace.Traces) error {
		var errs error
		resourceSpansSlice := traces.ResourceSpans()
		for i := 0; i < resourceSpansSlice.Len(); i++ {
			resourceSpans := resourceSpansSlice.At(i)
			_, err = exporter.ExportResourceSpans(ctx, resourceSpans)
			errs = multierr.Append(errs, err)
		}
		return errs
	})
}

func newElasticMetricsExporter(
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.MetricsExporter, error) {
	exporter, err := newElasticExporter(cfg.(*Config), set.Logger)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elastic APM metrics exporter: %w", err)
	}
	return exporterhelper.NewMetricsExporter(cfg, set, func(ctx context.Context, input pmetric.Metrics) error {
		var errs error
		resourceMetricsSlice := input.ResourceMetrics()
		for i := 0; i < resourceMetricsSlice.Len(); i++ {
			resourceMetrics := resourceMetricsSlice.At(i)
			_, err = exporter.ExportResourceMetrics(ctx, resourceMetrics)
			errs = multierr.Append(errs, err)
		}
		return errs
	})
}

type elasticExporter struct {
	transport transport.Transport
	logger    *zap.Logger
}

func newElasticExporter(config *Config, logger *zap.Logger) (*elasticExporter, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	transport, err := newTransport(config)
	if err != nil {
		return nil, err
	}
	return &elasticExporter{transport: transport, logger: logger}, nil
}

func newTransport(config *Config) (transport.Transport, error) {
	transport, err := transport.NewHTTPTransport()
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP transport: %w", err)
	}
	tlsConfig, err := config.LoadTLSConfig()
	if err != nil {
		return nil, err
	}
	httpTransport := transport.Client.Transport.(*http.Transport)
	httpTransport.TLSClientConfig = tlsConfig

	url, err := url.Parse(config.APMServerURL)
	if err != nil {
		return nil, err
	}
	transport.SetServerURL(url)

	if config.APIKey != "" {
		transport.SetAPIKey(config.APIKey)
	} else if config.SecretToken != "" {
		transport.SetSecretToken(config.SecretToken)
	}

	transport.SetUserAgent("opentelemetry-collector")
	return transport, nil
}

// ExportResourceSpans exports OTLP trace data to Elastic APM Server,
// returning the number of spans that were dropped along with any errors.
func (e *elasticExporter) ExportResourceSpans(ctx context.Context, rs ptrace.ResourceSpans) (int, error) {
	var w fastjson.Writer
	if err := elastic.EncodeResourceMetadata(rs.Resource(), &w); err != nil {
		return rs.ScopeSpans().Len(), err
	}

	var errs []error
	var count int
	scopeSpansSlice := rs.ScopeSpans()
	for i := 0; i < scopeSpansSlice.Len(); i++ {
		scopeSpans := scopeSpansSlice.At(i)
		scope := scopeSpans.Scope()
		spanSlice := scopeSpans.Spans()
		for i := 0; i < spanSlice.Len(); i++ {
			count++
			span := spanSlice.At(i)
			before := w.Size()
			if err := elastic.EncodeSpan(span, scope, rs.Resource(), &w); err != nil {
				w.Rewind(before)
				errs = append(errs, err)
			}
		}
	}
	if err := e.sendEvents(ctx, &w); err != nil {
		return count, err
	}
	return len(errs), multierr.Combine(errs...)
}

// ExportResourceMetrics exports OTLP metrics to Elastic APM Server,
// returning the number of metrics that were dropped along with any errors.
func (e *elasticExporter) ExportResourceMetrics(ctx context.Context, rm pmetric.ResourceMetrics) (int, error) {
	var w fastjson.Writer
	if err := elastic.EncodeResourceMetadata(rm.Resource(), &w); err != nil {
		return rm.ScopeMetrics().Len(), err
	}
	var errs error
	var totalDropped int
	scopeMetricsSlice := rm.ScopeMetrics()
	for i := 0; i < scopeMetricsSlice.Len(); i++ {
		scopeMetrics := scopeMetricsSlice.At(i)
		scope := scopeMetrics.Scope()
		metrics := scopeMetrics.Metrics()
		before := w.Size()
		dropped, err := elastic.EncodeMetrics(metrics, scope, &w)
		if err != nil {
			w.Rewind(before)
			errs = multierr.Append(errs, err)
		}
		totalDropped += dropped
	}
	if err := e.sendEvents(ctx, &w); err != nil {
		return totalDropped, err
	}
	return totalDropped, errs
}

func (e *elasticExporter) sendEvents(ctx context.Context, w *fastjson.Writer) error {
	e.logger.Debug("sending events", zap.ByteString("events", w.Bytes()))

	var buf bytes.Buffer
	zw, err := zlib.NewWriterLevel(&buf, zlib.DefaultCompression)
	if err != nil {
		return err
	}
	if _, err := zw.Write(w.Bytes()); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := e.transport.SendStream(ctx, &buf); err != nil {
		// TODO(axw) check response for number of accepted items,
		// and take that into account in the result.
		return err
	}
	return nil
}
