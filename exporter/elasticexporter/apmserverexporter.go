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

package elasticexporter

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter/internal/translator/elastic"
	"go.elastic.co/apm/transport"
	"go.elastic.co/fastjson"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// apmServerExporter sends traces and metrics to APM Server. The APM Server
// will apply additional processing, before indexing the actual events.
type apmServerExporter struct {
	apmServerExporter transport.Transport
	logger            *zap.Logger
}

// newAPMServerExporter creates an exporter that sends all data to APM Server.
func newAPMServerExporter(config *Config, logger *zap.Logger) (*apmServerExporter, error) {
	if err := config.APMServerConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %s", err)
	}
	transport, err := newAPMTransport(config)
	if err != nil {
		return nil, err
	}
	return &apmServerExporter{apmServerExporter: transport, logger: logger}, nil
}

func newAPMTransport(config *Config) (transport.Transport, error) {
	transport, err := transport.NewHTTPTransport()
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP transport: %v", err)
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
func (e *apmServerExporter) ExportResourceSpans(ctx context.Context, rs pdata.ResourceSpans) (int, error) {
	var w fastjson.Writer
	elastic.EncodeResourceMetadata(rs.Resource(), &w)
	var errs []error
	var count int
	instrumentationLibrarySpansSlice := rs.InstrumentationLibrarySpans()
	for i := 0; i < instrumentationLibrarySpansSlice.Len(); i++ {
		instrumentationLibrarySpans := instrumentationLibrarySpansSlice.At(i)
		instrumentationLibrary := instrumentationLibrarySpans.InstrumentationLibrary()
		spanSlice := instrumentationLibrarySpans.Spans()
		for i := 0; i < spanSlice.Len(); i++ {
			count++
			span := spanSlice.At(i)
			before := w.Size()
			if err := elastic.EncodeSpan(span, instrumentationLibrary, rs.Resource(), &w); err != nil {
				w.Rewind(before)
				errs = append(errs, err)
			}
		}
	}
	if err := e.sendEvents(ctx, &w); err != nil {
		return count, err
	}
	return len(errs), componenterror.CombineErrors(errs)
}

// ExportResourceMetrics exports OTLP metrics to Elastic APM Server,
// returning the number of metrics that were dropped along with any errors.
func (e *apmServerExporter) ExportResourceMetrics(ctx context.Context, rm pdata.ResourceMetrics) (int, error) {
	var w fastjson.Writer
	elastic.EncodeResourceMetadata(rm.Resource(), &w)
	var errs []error
	var totalDropped int
	instrumentationLibraryMetricsSlice := rm.InstrumentationLibraryMetrics()
	for i := 0; i < instrumentationLibraryMetricsSlice.Len(); i++ {
		instrumentationLibraryMetrics := instrumentationLibraryMetricsSlice.At(i)
		instrumentationLibrary := instrumentationLibraryMetrics.InstrumentationLibrary()
		metrics := instrumentationLibraryMetrics.Metrics()
		before := w.Size()
		dropped, err := elastic.EncodeMetrics(metrics, instrumentationLibrary, &w)
		if err != nil {
			w.Rewind(before)
			errs = append(errs, err)
		}
		totalDropped += dropped
	}
	if err := e.sendEvents(ctx, &w); err != nil {
		return totalDropped, err
	}
	return totalDropped, componenterror.CombineErrors(errs)
}

func (e *apmServerExporter) sendEvents(ctx context.Context, w *fastjson.Writer) error {
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
	if err := e.apmServerExporter.SendStream(ctx, &buf); err != nil {
		// TODO(axw) check response for number of accepted items,
		// and take that into account in the result.
		return err
	}
	return nil
}
