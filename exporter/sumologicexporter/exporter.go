// Copyright 2020 OpenTelemetry Authors
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

package sumologicexporter

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
)

type sumologicexporter struct {
	sources             sourceFormats
	config              *Config
	client              *http.Client
	filter              filter
	prometheusFormatter prometheusFormatter
	graphiteFormatter   graphiteFormatter
}

func initExporter(cfg *Config) (*sumologicexporter, error) {
	switch cfg.LogFormat {
	case JSONFormat:
	case TextFormat:
	default:
		return nil, fmt.Errorf("unexpected log format: %s", cfg.LogFormat)
	}

	switch cfg.MetricFormat {
	case GraphiteFormat:
	case Carbon2Format:
	case PrometheusFormat:
	default:
		return nil, fmt.Errorf("unexpected metric format: %s", cfg.MetricFormat)
	}

	switch cfg.CompressEncoding {
	case GZIPCompression:
	case DeflateCompression:
	case NoCompression:
	default:
		return nil, fmt.Errorf("unexpected compression encoding: %s", cfg.CompressEncoding)
	}

	if len(cfg.HTTPClientSettings.Endpoint) == 0 {
		return nil, errors.New("endpoint is not set")
	}

	sfs, err := newSourceFormats(cfg)
	if err != nil {
		return nil, err
	}

	f, err := newFilter(cfg.MetadataAttributes)
	if err != nil {
		return nil, err
	}

	pf, err := newPrometheusFormatter()
	if err != nil {
		return nil, err
	}

	gf, err := newGraphiteFormatter(cfg.GraphiteTemplate)
	if err != nil {
		return nil, err
	}

	se := &sumologicexporter{
		config:              cfg,
		sources:             sfs,
		filter:              f,
		prometheusFormatter: pf,
		graphiteFormatter:   gf,
	}

	return se, nil
}

func newLogsExporter(
	cfg *Config,
	set component.ExporterCreateSettings,
) (component.LogsExporter, error) {
	se, err := initExporter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the logs exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		cfg,
		set,
		se.pushLogsData,
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(se.start),
	)
}

func newMetricsExporter(
	cfg *Config,
	set component.ExporterCreateSettings,
) (component.MetricsExporter, error) {
	se, err := initExporter(cfg)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		cfg,
		set,
		se.pushMetricsData,
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(se.start),
	)
}

// start starts the exporter
func (se *sumologicexporter) start(_ context.Context, host component.Host) (err error) {
	client, err := se.config.HTTPClientSettings.ToClient(host.GetExtensions())
	if err != nil {
		return fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	se.client = client

	return nil
}

// pushLogsData groups data with common metadata and sends them as separate batched requests.
// It returns the number of unsent logs and an error which contains a list of dropped records
// so they can be handled by OTC retry mechanism
func (se *sumologicexporter) pushLogsData(ctx context.Context, ld pdata.Logs) error {
	var (
		currentMetadata  fields = newFields(pdata.NewAttributeMap())
		previousMetadata fields = newFields(pdata.NewAttributeMap())
		errs             error
		droppedRecords   []pdata.LogRecord
		err              error
	)

	c, err := newCompressor(se.config.CompressEncoding)
	if err != nil {
		return consumererror.NewLogs(fmt.Errorf("failed to initialize compressor: %w", err), ld)
	}
	sdr := newSender(
		se.config,
		se.client,
		se.filter,
		se.sources,
		c,
		se.prometheusFormatter,
		se.graphiteFormatter,
	)

	// Iterate over ResourceLogs
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)

		ills := rl.InstrumentationLibraryLogs()
		// iterate over InstrumentationLibraryLogs
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)

			// iterate over Logs
			logs := ill.Logs()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)

				// copy resource attributes into logs attributes
				// log attributes have precedence over resource attributes
				rl.Resource().Attributes().Range(func(k string, v pdata.AttributeValue) bool {
					log.Attributes().Insert(k, v)
					return true
				})

				currentMetadata = sdr.filter.filterIn(log.Attributes())

				// If metadata differs from currently buffered, flush the buffer
				if currentMetadata.string() != previousMetadata.string() && previousMetadata.string() != "" {
					var dropped []pdata.LogRecord
					dropped, err = sdr.sendLogs(ctx, previousMetadata)
					if err != nil {
						errs = multierr.Append(errs, err)
						droppedRecords = append(droppedRecords, dropped...)
					}
					sdr.cleanLogsBuffer()
				}

				// assign metadata
				previousMetadata = currentMetadata

				// add log to the buffer
				var dropped []pdata.LogRecord
				dropped, err = sdr.batchLog(ctx, log, previousMetadata)
				if err != nil {
					droppedRecords = append(droppedRecords, dropped...)
					errs = multierr.Append(errs, err)
				}
			}
		}
	}

	// Flush pending logs
	dropped, err := sdr.sendLogs(ctx, previousMetadata)
	if err != nil {
		droppedRecords = append(droppedRecords, dropped...)
		errs = multierr.Append(errs, err)
	}

	if len(droppedRecords) > 0 {
		// Move all dropped records to Logs
		droppedLogs := pdata.NewLogs()
		rls = droppedLogs.ResourceLogs()
		ills := rls.AppendEmpty().InstrumentationLibraryLogs()
		logs := ills.AppendEmpty().Logs()

		for _, log := range droppedRecords {
			tgt := logs.AppendEmpty()
			log.CopyTo(tgt)
		}

		return consumererror.NewLogs(errs, droppedLogs)
	}

	return nil
}

// pushMetricsData groups data with common metadata and send them as separate batched requests
// it returns number of unsent metrics and error which contains list of dropped records
// so they can be handle by the OTC retry mechanism
func (se *sumologicexporter) pushMetricsData(ctx context.Context, md pdata.Metrics) error {
	var (
		currentMetadata  fields = newFields(pdata.NewAttributeMap())
		previousMetadata fields = newFields(pdata.NewAttributeMap())
		errs             error
		droppedRecords   []metricPair
		attributes       pdata.AttributeMap
	)

	c, err := newCompressor(se.config.CompressEncoding)
	if err != nil {
		return consumererror.NewMetrics(fmt.Errorf("failed to initialize compressor: %w", err), md)
	}
	sdr := newSender(
		se.config,
		se.client,
		se.filter,
		se.sources,
		c,
		se.prometheusFormatter,
		se.graphiteFormatter,
	)

	// Iterate over ResourceMetrics
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		attributes = rm.Resource().Attributes()

		// iterate over InstrumentationLibraryMetrics
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)

			// iterate over Metrics
			ms := ilm.Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				mp := metricPair{
					metric:     m,
					attributes: attributes,
				}

				currentMetadata = sdr.filter.filterIn(attributes)

				// If metadata differs from currently buffered, flush the buffer
				if currentMetadata.string() != previousMetadata.string() && previousMetadata.string() != "" {
					var dropped []metricPair
					dropped, err = sdr.sendMetrics(ctx, previousMetadata)
					if err != nil {
						errs = multierr.Append(errs, err)
						droppedRecords = append(droppedRecords, dropped...)
					}
					sdr.cleanMetricBuffer()
				}

				// assign metadata
				previousMetadata = currentMetadata
				var dropped []metricPair
				// add metric to the buffer
				dropped, err = sdr.batchMetric(ctx, mp, currentMetadata)
				if err != nil {
					droppedRecords = append(droppedRecords, dropped...)
					errs = multierr.Append(errs, err)
				}
			}
		}
	}

	// Flush pending metrics
	dropped, err := sdr.sendMetrics(ctx, previousMetadata)
	if err != nil {
		droppedRecords = append(droppedRecords, dropped...)
		errs = multierr.Append(errs, err)
	}

	if len(droppedRecords) > 0 {
		// Move all dropped records to Metrics
		droppedMetrics := pdata.NewMetrics()
		rms := droppedMetrics.ResourceMetrics()
		rms.EnsureCapacity(len(droppedRecords))
		for _, record := range droppedRecords {
			rm := droppedMetrics.ResourceMetrics().AppendEmpty()
			record.attributes.CopyTo(rm.Resource().Attributes())

			ilms := rm.InstrumentationLibraryMetrics()
			record.metric.CopyTo(ilms.AppendEmpty().Metrics().AppendEmpty())
		}

		return consumererror.NewMetrics(errs, droppedMetrics)
	}

	return nil
}
