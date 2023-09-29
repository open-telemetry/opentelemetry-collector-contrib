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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/SumoLogic/sumologic-otel-collector/pkg/extension/sumologicextension"
)

const (
	logsDataUrl    = "/api/v1/collector/logs"
	metricsDataUrl = "/api/v1/collector/metrics"
	tracesDataUrl  = "/api/v1/collector/traces"
)

type sumologicexporter struct {
	config *Config
	host   component.Host
	logger *zap.Logger

	clientLock sync.RWMutex
	client     *http.Client

	compressorPool sync.Pool

	prometheusFormatter prometheusFormatter

	// Lock around data URLs is needed because the reconfiguration of the exporter
	// can happen asynchronously whenever the exporter is re registering.
	dataUrlsLock   sync.RWMutex
	dataUrlMetrics string
	dataUrlLogs    string
	dataUrlTraces  string

	id component.ID
}

func initExporter(cfg *Config, createSettings exporter.CreateSettings) (*sumologicexporter, error) {
	pf, err := newPrometheusFormatter()
	if err != nil {
		return nil, err
	}

	se := &sumologicexporter{
		config: cfg,
		logger: createSettings.Logger,
		compressorPool: sync.Pool{
			New: func() any {
				c, err := newCompressor(cfg.CompressEncoding)
				if err != nil {
					return fmt.Errorf("failed to initialize compressor: %w", err)
				}
				return &c
			},
		},
		// NOTE: client is now set in start()
		prometheusFormatter: pf,
		id:                  createSettings.ID,
	}

	se.logger.Info(
		"Sumo Logic Exporter configured",
		zap.String("log_format", string(cfg.LogFormat)),
		zap.String("metric_format", string(cfg.MetricFormat)),
		zap.String("trace_format", string(cfg.TraceFormat)),
	)

	return se, nil
}

func newLogsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg *Config,
) (exporter.Logs, error) {
	se, err := initExporter(cfg, params)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the logs exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		params,
		cfg,
		se.pushLogsData,
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(se.start),
		exporterhelper.WithShutdown(se.shutdown),
	)
}

func newMetricsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg *Config,
) (exporter.Metrics, error) {
	se, err := initExporter(cfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		ctx,
		params,
		cfg,
		se.pushMetricsData,
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(se.start),
		exporterhelper.WithShutdown(se.shutdown),
	)
}

func newTracesExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg *Config,
) (exporter.Traces, error) {
	se, err := initExporter(cfg, params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		params,
		cfg,
		se.pushTracesData,
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(se.start),
		exporterhelper.WithShutdown(se.shutdown),
	)
}

// pushLogsData groups data with common metadata and sends them as separate batched requests.
// It returns the number of unsent logs and an error which contains a list of dropped records
// so they can be handled by OTC retry mechanism
func (se *sumologicexporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	compr, err := se.getCompressor()
	if err != nil {
		return consumererror.NewLogs(err, ld)
	}
	defer se.compressorPool.Put(compr)

	logsUrl, metricsUrl, tracesUrl := se.getDataURLs()
	sdr := newSender(
		se.logger,
		se.config,
		se.getHTTPClient(),
		compr,
		se.prometheusFormatter,
		metricsUrl,
		logsUrl,
		tracesUrl,
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

		se.dropRoutingAttribute(rl.Resource().Attributes())
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

		// Move all dropped records to Logs
		// NOTE: we only copy resource and log records here.
		// Scope is not handled properly but it never was.
		for i := range dropped {
			rls := ld.ResourceLogs().AppendEmpty()
			dropped[i].resource.MoveTo(rls.Resource())

			for j := 0; j < len(dropped[i].records); j++ {
				dropped[i].records[j].MoveTo(
					rls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty(),
				)
			}
		}

		errs = deduplicateErrors(errs)
		se.handleUnauthorizedErrors(ctx, errs...)
		return consumererror.NewLogs(multierr.Combine(errs...), ld)
	}

	return nil
}

// pushMetricsData groups data with common metadata and send them as separate batched requests
// it returns number of unsent metrics and error which contains list of dropped records
// so they can be handle by the OTC retry mechanism
func (se *sumologicexporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	compr, err := se.getCompressor()
	if err != nil {
		return consumererror.NewMetrics(err, md)
	}
	defer se.compressorPool.Put(compr)

	logsUrl, metricsUrl, tracesUrl := se.getDataURLs()
	sdr := newSender(
		se.logger,
		se.config,
		se.getHTTPClient(),
		compr,
		se.prometheusFormatter,
		metricsUrl,
		logsUrl,
		tracesUrl,
		se.id,
	)

	// Transform metrics metadata
	// this includes dropping the routing attribute and translating attributes
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		se.dropRoutingAttribute(rm.Resource().Attributes())
	}

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
		return consumererror.NewMetrics(multierr.Combine(errs...), droppedMetrics)
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

func (se *sumologicexporter) pushTracesData(ctx context.Context, td ptrace.Traces) error {
	compr, err := se.getCompressor()
	if err != nil {
		return consumererror.NewTraces(err, td)
	}
	defer se.compressorPool.Put(compr)

	logsUrl, metricsUrl, tracesUrl := se.getDataURLs()
	sdr := newSender(
		se.logger,
		se.config,
		se.getHTTPClient(),
		compr,
		se.prometheusFormatter,
		metricsUrl,
		logsUrl,
		tracesUrl,
		se.id,
	)

	// Drop routing attribute from ResourceSpans
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		se.dropRoutingAttribute(rss.At(i).Resource().Attributes())
	}

	err = sdr.sendTraces(ctx, td)
	se.handleUnauthorizedErrors(ctx, err)
	return err
}

func (se *sumologicexporter) getCompressor() (*compressor, error) {
	switch c := se.compressorPool.Get().(type) {
	case error:
		return &compressor{}, fmt.Errorf("%v", c)
	case *compressor:
		return c, nil
	default:
		return &compressor{}, fmt.Errorf("unknown compressor type: %T", c)
	}
}

func (se *sumologicexporter) start(ctx context.Context, host component.Host) error {
	se.host = host
	return se.configure(ctx)
}

func (se *sumologicexporter) configure(ctx context.Context) error {
	var (
		ext          *sumologicextension.SumologicExtension
		foundSumoExt bool
	)

	httpSettings := se.config.HTTPClientSettings

	for _, e := range se.host.GetExtensions() {
		v, ok := e.(*sumologicextension.SumologicExtension)
		if ok && httpSettings.Auth.AuthenticatorID == v.ComponentID() {
			ext = v
			foundSumoExt = true
			break
		}
	}

	if httpSettings.Endpoint == "" && httpSettings.Auth != nil &&
		string(httpSettings.Auth.AuthenticatorID.Type()) == "sumologic" {
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

		u, err := url.Parse(ext.BaseUrl())
		if err != nil {
			return fmt.Errorf("failed to parse API base URL from sumologicextension: %w", err)
		}

		logsUrl := *u
		logsUrl.Path = logsDataUrl
		metricsUrl := *u
		metricsUrl.Path = metricsDataUrl
		tracesUrl := *u
		tracesUrl.Path = tracesDataUrl
		se.setDataURLs(logsUrl.String(), metricsUrl.String(), tracesUrl.String())

	} else if httpSettings.Endpoint != "" {
		logsUrl, err := getSignalURL(se.config, httpSettings.Endpoint, component.DataTypeLogs)
		if err != nil {
			return err
		}
		metricsUrl, err := getSignalURL(se.config, httpSettings.Endpoint, component.DataTypeMetrics)
		if err != nil {
			return err
		}
		tracesUrl, err := getSignalURL(se.config, httpSettings.Endpoint, component.DataTypeTraces)
		if err != nil {
			return err
		}
		se.setDataURLs(logsUrl, metricsUrl, tracesUrl)

		// Clean authenticator if set to sumologic.
		// Setting to null in configuration doesn't work, so we have to force it that way.
		if httpSettings.Auth != nil && string(httpSettings.Auth.AuthenticatorID.Type()) == "sumologic" {
			httpSettings.Auth = nil
		}
	} else {
		return fmt.Errorf("no auth extension and no endpoint specified")
	}

	client, err := httpSettings.ToClient(se.host, component.TelemetrySettings{})
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
	se.dataUrlsLock.Lock()
	se.logger.Info("setting data urls", zap.String("logs_url", logs), zap.String("metrics_url", metrics), zap.String("traces_url", traces))
	se.dataUrlLogs, se.dataUrlMetrics, se.dataUrlTraces = logs, metrics, traces
	se.dataUrlsLock.Unlock()
}

func (se *sumologicexporter) getDataURLs() (logs, metrics, traces string) {
	se.dataUrlsLock.RLock()
	defer se.dataUrlsLock.RUnlock()
	return se.dataUrlLogs, se.dataUrlMetrics, se.dataUrlTraces
}

func (se *sumologicexporter) shutdown(context.Context) error {
	return nil
}

func (se *sumologicexporter) dropRoutingAttribute(attr pcommon.Map) {
	attr.Remove(se.config.DropRoutingAttribute)
}

// get the destination url for a given signal type
// this mostly adds signal-specific suffixes if the format is otlp
func getSignalURL(oCfg *Config, endpointUrl string, signal component.DataType) (string, error) {
	url, err := url.Parse(endpointUrl)
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

	signalUrlSuffix := fmt.Sprintf("/v1/%s", signal)
	if !strings.HasSuffix(url.Path, signalUrlSuffix) {
		url.Path = path.Join(url.Path, signalUrlSuffix)
	}

	return url.String(), nil
}
