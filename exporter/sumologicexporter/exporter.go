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

	if cfg.MetricFormat == RemovedGraphiteFormat {
		settings.Logger.Error("`metric_format: graphite` nad `graphite_template` are no longer supported. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
	}

	if cfg.MetricFormat == RemovedCarbon2Format {
		settings.Logger.Error("`metric_format: carbon` is no longer supported. See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/sumologicexporter#migration-to-new-architecture for more information")
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

	se := &sumologicexporter{
		logger:              settings.Logger,
		config:              cfg,
		sources:             sfs,
		filter:              f,
		prometheusFormatter: pf,
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
		se.config.ClientConfig.Compression = se.config.CompressEncoding
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
	logsURL, metricsURL, tracesURL := se.getDataURLs()
	sdr := newSender(
		se.logger,
		se.config,
		se.getHTTPClient(),
		se.filter,
		se.sources,
		se.prometheusFormatter,
		metricsURL,
		logsURL,
		tracesURL,
		se.id,
	)

	// Follow different execution path for OTLP format
	if sdr.config.LogFormat == OTLPLogFormat {
		if err := sdr.sendOTLPLogs(ctx, ld); err != nil {
			se.handleUnauthorizedErrors(ctx, err)
			return consumererror.NewLogs(err, ld)
		}
		return nil
	}

	type droppedResourceRecords struct {
		resource pcommon.Resource
		records  []plog.LogRecord
	}
	var (
		errs    []error
		dropped []droppedResourceRecords
	)

	// Iterate over ResourceLogs
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)

		currentMetadata := newFields(rl.Resource().Attributes())

		if droppedRecords, err := sdr.sendNonOTLPLogs(ctx, rl, currentMetadata); err != nil {
			dropped = append(dropped, droppedResourceRecords{
				resource: rl.Resource(),
				records:  droppedRecords,
			})
			errs = append(errs, err)
		}
	}

	if len(dropped) > 0 {
		ld = plog.NewLogs()

		// Copy all dropped records to Logs
		// NOTE: we only copy resource and log records here.
		// Scope is not handled properly but it never was.
		for i := range dropped {
			rls := ld.ResourceLogs().AppendEmpty()
			dropped[i].resource.CopyTo(rls.Resource())

			for j := 0; j < len(dropped[i].records); j++ {
				dropped[i].records[j].CopyTo(
					rls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty(),
				)
			}
		}

		errs = deduplicateErrors(errs)
		se.handleUnauthorizedErrors(ctx, errs...)
		return consumererror.NewLogs(errors.Join(errs...), ld)
	}

	return nil
}

// pushMetricsData groups data with common metadata and send them as separate batched requests
// it returns number of unsent metrics and error which contains list of dropped records
// so they can be handle by the OTC retry mechanism
func (se *sumologicexporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	logsURL, metricsURL, tracesURL := se.getDataURLs()
	sdr := newSender(
		se.logger,
		se.config,
		se.getHTTPClient(),
		se.filter,
		se.sources,
		se.prometheusFormatter,
		metricsURL,
		logsURL,
		tracesURL,
		se.id,
	)

	var droppedMetrics pmetric.Metrics
	var errs []error
	if sdr.config.MetricFormat == OTLPMetricFormat {
		if err := sdr.sendOTLPMetrics(ctx, md); err != nil {
			droppedMetrics = md
			errs = []error{err}
		}
	} else {
		droppedMetrics, errs = sdr.sendNonOTLPMetrics(ctx, md)
	}

	if len(errs) > 0 {
		se.handleUnauthorizedErrors(ctx, errs...)
		return consumererror.NewMetrics(errors.Join(errs...), droppedMetrics)
	}

	return nil
}

// handleUnauthorizedErrors checks if any of the provided errors is an unauthorized error.
// In which case it triggers exporter reconfiguration which in turn takes the credentials
// from sumologicextension which at this point should already detect the problem with
// authorization (via heartbeats) and prepare new collector credentials to be available.
func (se *sumologicexporter) handleUnauthorizedErrors(ctx context.Context, errs ...error) {
	for _, err := range errs {
		if errors.Is(err, errUnauthorized) {
			se.logger.Warn("Received unauthorized status code, triggering reconfiguration")
			if errC := se.configure(ctx); errC != nil {
				se.logger.Error("Error configuring the exporter with new credentials", zap.Error(err))
			} else {
				// It's enough to successfully reconfigure the exporter just once.
				return
			}
		}
	}
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
