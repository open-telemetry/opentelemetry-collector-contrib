// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"
)

const (
	logsDataURL    = "/api/v1/collector/logs"
	metricsDataURL = "/api/v1/collector/metrics"
	tracesDataURL  = "/api/v1/collector/traces"
)

type sumologicexporter struct {
	config *Config
	host   component.Host
	logger *zap.Logger

	sources             sourceFormats
	filter              filter
	prometheusFormatter prometheusFormatter
	graphiteFormatter   graphiteFormatter
	settings            component.TelemetrySettings

	clientLock sync.RWMutex
	client     *http.Client

	// Lock around data URLs is needed because the reconfiguration of the exporter
	// can happen asynchronously whenever the exporter is re registering.
	dataURLsLock   sync.RWMutex
	dataURLMetrics string
	dataURLLogs    string
	dataURLTraces  string

	foundSumologicExtension bool
	sumologicExtension      *sumologicextension.SumologicExtension

	id component.ID
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
		logger:              settings.Logger,
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
		exporterhelper.WithShutdown(se.shutdown),
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
		exporterhelper.WithShutdown(se.shutdown),
	)
}

// start starts the exporter
func (se *sumologicexporter) start(ctx context.Context, host component.Host) (err error) {
	se.host = host
	return se.configure(ctx)
}

func (se *sumologicexporter) configure(ctx context.Context) error {
	var (
		ext          *sumologicextension.SumologicExtension
		foundSumoExt bool
	)

	if se.config.CompressEncoding != NoCompression {
		se.config.ClientConfig.Compression = configcompression.Type(se.config.CompressEncoding)
	}

	httpSettings := se.config.ClientConfig

	for _, e := range se.host.GetExtensions() {
		v, ok := e.(*sumologicextension.SumologicExtension)
		if ok && httpSettings.Auth.AuthenticatorID == v.ComponentID() {
			ext = v
			foundSumoExt = true
			se.foundSumologicExtension = true
			se.sumologicExtension = ext
			break
		}
	}

	switch {
	case httpSettings.Endpoint == "" && httpSettings.Auth != nil &&
		httpSettings.Auth.AuthenticatorID.Type() == sumologicextension.NewFactory().Type():
		// If user specified using sumologicextension as auth but none was
		// found then return an error.
		if !foundSumoExt {
			return fmt.Errorf(
				"sumologic was specified as auth extension (named: %q) but "+
					"a matching extension was not found in the config, "+
					"please re-check the config and/or define the sumologicextension",
				httpSettings.Auth.AuthenticatorID.String(),
			)
		}

		// If we're using sumologicextension as authentication extension and
		// endpoint was not set then send data on a collector generic ingest URL
		// with authentication set by sumologicextension.

		u, err := url.Parse(ext.BaseURL())
		if err != nil {
			return fmt.Errorf("failed to parse API base URL from sumologicextension: %w", err)
		}

		logsURL := *u
		logsURL.Path = logsDataURL
		metricsURL := *u
		metricsURL.Path = metricsDataURL
		tracesURL := *u
		tracesURL.Path = tracesDataURL
		se.setDataURLs(logsURL.String(), metricsURL.String(), tracesURL.String())

	case httpSettings.Endpoint != "":
		logsURL, err := getSignalURL(se.config, httpSettings.Endpoint, component.DataTypeLogs)
		if err != nil {
			return err
		}
		metricsURL, err := getSignalURL(se.config, httpSettings.Endpoint, component.DataTypeMetrics)
		if err != nil {
			return err
		}
		tracesURL, err := getSignalURL(se.config, httpSettings.Endpoint, component.DataTypeTraces)
		if err != nil {
			return err
		}
		se.setDataURLs(logsURL, metricsURL, tracesURL)

		// Clean authenticator if set to sumologic.
		// Setting to null in configuration doesn't work, so we have to force it that way.
		if httpSettings.Auth != nil && httpSettings.Auth.AuthenticatorID.Type() == sumologicextension.NewFactory().Type() {
			httpSettings.Auth = nil
		}
	default:
		return fmt.Errorf("no auth extension and no endpoint specified")
	}

	client, err := httpSettings.ToClient(ctx, se.host, component.TelemetrySettings{})
	if err != nil {
		return fmt.Errorf("failed to create HTTP Client: %w", err)
	}

	se.setHTTPClient(client)
	return nil
}

func (se *sumologicexporter) setHTTPClient(client *http.Client) {
	se.clientLock.Lock()
	se.client = client
	se.clientLock.Unlock()
}

func (se *sumologicexporter) getHTTPClient() *http.Client {
	se.clientLock.RLock()
	defer se.clientLock.RUnlock()
	return se.client
}

func (se *sumologicexporter) setDataURLs(logs, metrics, traces string) {
	se.dataURLsLock.Lock()
	se.logger.Info("setting data urls", zap.String("logs_url", logs), zap.String("metrics_url", metrics), zap.String("traces_url", traces))
	se.dataURLLogs, se.dataURLMetrics, se.dataURLTraces = logs, metrics, traces
	se.dataURLsLock.Unlock()
}

func (se *sumologicexporter) getDataURLs() (logs, metrics, traces string) {
	se.dataURLsLock.RLock()
	defer se.dataURLsLock.RUnlock()
	return se.dataURLLogs, se.dataURLMetrics, se.dataURLTraces
}

func (se *sumologicexporter) shutdown(context.Context) error {
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
	logsURL, metricsURL, tracesURL := se.getDataURLs()
	sdr := newSender(
		se.logger,
		se.config,
		se.getHTTPClient(),
		se.filter,
		se.sources,
		c,
		se.prometheusFormatter,
		metricsURL,
		logsURL,
		tracesURL,
		se.graphiteFormatter,
		se.id,
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
	logsURL, metricsURL, tracesURL := se.getDataURLs()
	sdr := newSender(
		se.logger,
		se.config,
		se.getHTTPClient(),
		se.filter,
		se.sources,
		c,
		se.prometheusFormatter,
		metricsURL,
		logsURL,
		tracesURL,
		se.graphiteFormatter,
		se.id,
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

// get the destination url for a given signal type
// this mostly adds signal-specific suffixes if the format is otlp
func getSignalURL(oCfg *Config, endpointURL string, signal component.DataType) (string, error) {
	url, err := url.Parse(endpointURL)
	if err != nil {
		return "", err
	}

	switch signal {
	case component.DataTypeLogs:
		if oCfg.LogFormat != "otlp" {
			return url.String(), nil
		}
	case component.DataTypeMetrics:
		if oCfg.MetricFormat != "otlp" {
			return url.String(), nil
		}
	case component.DataTypeTraces:
	default:
		return "", fmt.Errorf("unknown signal type: %s", signal)
	}

	signalURLSuffix := fmt.Sprintf("/v1/%s", signal)
	if !strings.HasSuffix(url.Path, signalURLSuffix) {
		url.Path = path.Join(url.Path, signalURLSuffix)
	}

	return url.String(), nil
}
