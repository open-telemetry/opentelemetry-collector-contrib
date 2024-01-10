// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type sumologicexporter struct {
	sources             sourceFormats
	config              *Config
	client              *http.Client
	filter              filter
	prometheusFormatter prometheusFormatter
	graphiteFormatter   graphiteFormatter
	settings            component.TelemetrySettings
}

func initExporter(cfg *Config, settings component.TelemetrySettings) (*sumologicexporter, error) {

	if cfg.MetricFormat == GraphiteFormat {
		settings.Logger.Warn("`metric_format: graphite` nad `graphite_template` are deprecated and are going to be removed in the future. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
	}

	if cfg.MetricFormat == Carbon2Format {
		settings.Logger.Warn("`metric_format: carbon` is deprecated and is going to be removed in the future. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
	}

	if len(cfg.MetadataAttributes) > 0 {
		settings.Logger.Warn("`metadata_attributes: []` is deprecated and is going to be removed in the future. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
	}

	if cfg.SourceCategory != "" {
		settings.Logger.Warn("`source_category: <template>` is deprecated and is going to be removed in the future. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
	}

	if cfg.SourceHost != "" {
		settings.Logger.Warn("`source_host: <template>` is deprecated and is going to be removed in the future. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
	}

	if cfg.SourceName != "" {
		settings.Logger.Warn("`source_name: <template>` is deprecated and is going to be removed in the future. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
	}

	sfs := newSourceFormats(cfg)

	f, err := newFilter(cfg.MetadataAttributes)
	if err != nil {
		return nil, err
	}

	pf := newPrometheusFormatter()

	gf := newGraphiteFormatter(cfg.GraphiteTemplate)

	se := &sumologicexporter{
		config:              cfg,
		sources:             sfs,
		filter:              f,
		prometheusFormatter: pf,
		graphiteFormatter:   gf,
		settings:            settings,
	}

	return se, nil
}

func newLogsExporter(
	cfg *Config,
	set exporter.CreateSettings,
) (exporter.Logs, error) {
	se, err := initExporter(cfg, set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the logs exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		context.TODO(),
		set,
		cfg,
		se.pushLogsData,
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(se.start),
	)
}

func newMetricsExporter(
	cfg *Config,
	set exporter.CreateSettings,
) (exporter.Metrics, error) {
	se, err := initExporter(cfg, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		context.TODO(),
		set,
		cfg,
		se.pushMetricsData,
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(se.start),
	)
}

// start starts the exporter
func (se *sumologicexporter) start(_ context.Context, host component.Host) (err error) {
	client, err := se.config.HTTPClientSettings.ToClient(host, se.settings)
	if err != nil {
		return fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	se.client = client

	return nil
}

// pushLogsData groups data with common metadata and sends them as separate batched requests.
// It returns the number of unsent logs and an error which contains a list of dropped records
// so they can be handled by OTC retry mechanism
func (se *sumologicexporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	var (
		currentMetadata  fields
		previousMetadata = newFields(pcommon.NewMap())
		errs             []error
		droppedRecords   []plog.LogRecord
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

		ills := rl.ScopeLogs()
		// iterate over ScopeLogs
		for j := 0; j < ills.Len(); j++ {
			sl := ills.At(j)

			// iterate over Logs
			logs := sl.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)

				currentMetadata = sdr.filter.mergeAndFilterIn(rl.Resource().Attributes(), log.Attributes())

				// If metadata differs from currently buffered, flush the buffer
				if currentMetadata.string() != previousMetadata.string() && previousMetadata.string() != "" {
					var dropped []plog.LogRecord
					dropped, err = sdr.sendLogs(ctx, previousMetadata)
					if err != nil {
						errs = append(errs, err)
						droppedRecords = append(droppedRecords, dropped...)
					}
					sdr.cleanLogsBuffer()
				}

				// assign metadata
				previousMetadata = currentMetadata

				// add log to the buffer
				var dropped []plog.LogRecord
				dropped, err = sdr.batchLog(ctx, log, previousMetadata)
				if err != nil {
					droppedRecords = append(droppedRecords, dropped...)
					errs = append(errs, err)
				}
			}
		}
	}

	// Flush pending logs
	dropped, err := sdr.sendLogs(ctx, previousMetadata)
	if err != nil {
		droppedRecords = append(droppedRecords, dropped...)
		errs = append(errs, err)
	}

	if len(droppedRecords) > 0 {
		// Move all dropped records to Logs
		droppedLogs := plog.NewLogs()
		rls = droppedLogs.ResourceLogs()
		ills := rls.AppendEmpty().ScopeLogs()
		logs := ills.AppendEmpty().LogRecords()

		for _, log := range droppedRecords {
			tgt := logs.AppendEmpty()
			log.CopyTo(tgt)
		}

		return consumererror.NewLogs(errors.Join(errs...), droppedLogs)
	}

	return nil
}

// pushMetricsData groups data with common metadata and send them as separate batched requests
// it returns number of unsent metrics and error which contains list of dropped records
// so they can be handle by the OTC retry mechanism
func (se *sumologicexporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	var (
		currentMetadata  fields
		previousMetadata = newFields(pcommon.NewMap())
		errs             []error
		droppedRecords   []metricPair
		attributes       pcommon.Map
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

		// iterate over ScopeMetrics
		ilms := rm.ScopeMetrics()
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

				currentMetadata = sdr.filter.mergeAndFilterIn(attributes)

				// If metadata differs from currently buffered, flush the buffer
				if currentMetadata.string() != previousMetadata.string() && previousMetadata.string() != "" {
					var dropped []metricPair
					dropped, err = sdr.sendMetrics(ctx, previousMetadata)
					if err != nil {
						errs = append(errs, err)
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
					errs = append(errs, err)
				}
			}
		}
	}

	// Flush pending metrics
	dropped, err := sdr.sendMetrics(ctx, previousMetadata)
	if err != nil {
		droppedRecords = append(droppedRecords, dropped...)
		errs = append(errs, err)
	}

	if len(droppedRecords) > 0 {
		// Move all dropped records to Metrics
		droppedMetrics := pmetric.NewMetrics()
		rms := droppedMetrics.ResourceMetrics()
		rms.EnsureCapacity(len(droppedRecords))
		for _, record := range droppedRecords {
			rm := droppedMetrics.ResourceMetrics().AppendEmpty()
			record.attributes.CopyTo(rm.Resource().Attributes())

			ilms := rm.ScopeMetrics()
			record.metric.CopyTo(ilms.AppendEmpty().Metrics().AppendEmpty())
		}

		return consumererror.NewMetrics(errors.Join(errs...), droppedMetrics)
	}

	return nil
}
