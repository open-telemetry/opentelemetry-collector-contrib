// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/DataDog/agent-payload/v5/gogen"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator/header"
)

type datadogReceiver struct {
	address            string
	config             *Config
	params             receiver.Settings
	intakeReverseProxy *httputil.ReverseProxy

	nextTracesConsumer  consumer.Traces
	nextMetricsConsumer consumer.Metrics
	nextLogsConsumer    consumer.Logs

	metricsTranslator *translator.MetricsTranslator
	statsTranslator   *translator.StatsTranslator

	server    *http.Server
	tReceiver *receiverhelper.ObsReport

	traceIDCache *lru.Cache[uint64, pcommon.TraceID]
}

// Endpoint specifies an API endpoint definition.
type endpoint struct {
	// Pattern specifies the API pattern, as registered by the HTTP handler.
	Pattern string

	// Handler specifies the http.Handler for this endpoint.
	Handler func(http.ResponseWriter, *http.Request)
}

// getEndpoints specifies the list of endpoints registered for the trace-agent API.
func (ddr *datadogReceiver) getEndpoints() []endpoint {
	endpoints := []endpoint{
		{
			Pattern: "/",
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	if ddr.nextTracesConsumer != nil {
		endpoints = append(endpoints, []endpoint{
			{
				Pattern: "/v0.3/traces",
				Handler: ddr.handleTraces,
			},
			{
				Pattern: "/v0.4/traces",
				Handler: ddr.handleTraces,
			},
			{
				Pattern: "/v0.5/traces",
				Handler: ddr.handleTraces,
			},
			{
				Pattern: "/v0.7/traces",
				Handler: ddr.handleTraces,
			},
			{
				Pattern: "/api/v0.2/traces",
				Handler: ddr.handleTraces,
			},
		}...)
	}

	if ddr.nextMetricsConsumer != nil {
		endpoints = append(endpoints, []endpoint{
			{
				Pattern: "/api/v1/series",
				Handler: ddr.handleV1Series,
			},
			{
				Pattern: "/api/v2/series",
				Handler: ddr.handleV2Series,
			},
			{
				Pattern: "/api/v1/check_run",
				Handler: ddr.handleCheckRun,
			},
			{
				Pattern: "/api/v1/sketches",
				Handler: ddr.handleSketches,
			},
			{
				Pattern: "/api/beta/sketches",
				Handler: ddr.handleSketches,
			},
			{
				Pattern: "/intake",
				Handler: ddr.handleIntake,
			},
			{
				// the datadog agent is configured to use a trailing slash in some places:
				// https://github.com/DataDog/datadog-agent/blob/7.64.3/comp/forwarder/defaultforwarder/endpoints/endpoints.go#L18
				Pattern: "/intake/",
				Handler: ddr.handleIntake,
			},
			{
				Pattern: "/api/v1/distribution_points",
				Handler: ddr.handleDistributionPoints,
			},
			{
				Pattern: "/v0.6/stats",
				Handler: ddr.handleStats,
			},
			{
				Pattern: "/api/v0.2/stats",
				Handler: ddr.handleStatsV2,
			},
		}...)
	}

	if ddr.nextLogsConsumer != nil {
		endpoints = append(endpoints, []endpoint{
			{
				Pattern: "/api/v2/logs",
				Handler: ddr.handleLogs,
			},
		}...)
	}

	infoResponse, _ := ddr.buildInfoResponse(endpoints)

	endpoints = append(endpoints, endpoint{
		Pattern: "/info",
		Handler: func(w http.ResponseWriter, r *http.Request) { ddr.handleInfo(w, r, infoResponse) },
	})

	return endpoints
}

func newDataDogReceiver(ctx context.Context, config *Config, params receiver.Settings) (component.Component, error) {
	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	var cache *lru.Cache[uint64, pcommon.TraceID]
	if FullTraceIDFeatureGate.IsEnabled() {
		cache, err = lru.NewWithEvict(config.TraceIDCacheSize, func(k uint64, _ pcommon.TraceID) {
			params.Logger.Debug("evicting datadog trace id from cache", zap.Uint64("id", k))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create traceID cache: %w", err)
		}
	}

	var intakeReverseProxy *httputil.ReverseProxy
	if config.Intake.Behavior == configIntakeBehaviorProxy {
		datadogSite := config.Intake.Proxy.API.Site
		if datadogSite == "" {
			datadogSite = defaultConfigIntakeProxyAPISite
		}
		intakeReverseProxy = &httputil.ReverseProxy{
			Director: createIntakeReverseProxyDirector(datadogSite, string(config.Intake.Proxy.API.Key)),
		}
		if config.Intake.Proxy.API.FailOnInvalidKey {
			apiClient := clientutil.CreateAPIClient(
				params.BuildInfo,
				fmt.Sprintf("https://api.%s", datadogSite),
				confighttp.NewDefaultClientConfig(),
			)
			err := clientutil.ValidateAPIKey(ctx, string(config.Intake.Proxy.API.Key), params.Logger, apiClient)
			if err != nil {
				return nil, err
			}
		}
	}

	return &datadogReceiver{
		params:             params,
		config:             config,
		intakeReverseProxy: intakeReverseProxy,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		tReceiver:         instance,
		metricsTranslator: translator.NewMetricsTranslator(params.BuildInfo),
		statsTranslator:   translator.NewStatsTranslator(),
		traceIDCache:      cache,
	}, nil
}

func (ddr *datadogReceiver) Start(ctx context.Context, host component.Host) error {
	ddmux := http.NewServeMux()
	endpoints := ddr.getEndpoints()

	for _, e := range endpoints {
		ddmux.HandleFunc(e.Pattern, e.Handler)
	}

	var err error
	ddr.server, err = ddr.config.ToServer(
		ctx,
		host.GetExtensions(),
		ddr.params.TelemetrySettings,
		ddmux,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := ddr.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	ddr.address = hln.Addr().String()

	go func() {
		if err := ddr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting datadog receiver: %w", err)))
		}
	}()
	return nil
}

func (ddr *datadogReceiver) Shutdown(ctx context.Context) (err error) {
	return ddr.server.Shutdown(ctx)
}

func (ddr *datadogReceiver) buildInfoResponse(endpoints []endpoint) ([]byte, error) {
	var endpointPaths []string
	for _, e := range endpoints {
		endpointPaths = append(endpointPaths, e.Pattern)
	}

	return json.MarshalIndent(translator.DDInfo{
		Version:          fmt.Sprintf("datadogreceiver-%s-%s", ddr.params.BuildInfo.Command, ddr.params.BuildInfo.Version),
		Endpoints:        endpointPaths,
		ClientDropP0s:    false,
		SpanMetaStructs:  false,
		LongRunningSpans: false,
	}, "", "\t")
}

// handleInfo handles incoming /info payloads.
func (ddr *datadogReceiver) handleInfo(w http.ResponseWriter, _ *http.Request, infoResponse []byte) {
	_, err := fmt.Fprintf(w, "%s", infoResponse)
	if err != nil {
		ddr.params.Logger.Error("Error writing /info endpoint response", zap.Error(err))
		http.Error(w, "Error writing /info endpoint response", http.StatusInternalServerError)
		return
	}
}

func (ddr *datadogReceiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	if req.ContentLength == 0 { // Ping mechanism of Datadog SDK perform http request with empty body when GET /info not implemented.
		http.Error(w, "Fake featuresdiscovery", http.StatusBadRequest) // The response code should be different of 404 to be considered ok by Datadog SDK.
		return
	}
	obsCtx := ddr.tReceiver.StartLogsOp(req.Context())
	var (
		logCount int
		err      error
	)
	defer func(logCount *int) {
		ddr.tReceiver.EndLogsOp(obsCtx, "datadog", *logCount, err)
	}(&logCount)

	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed, supported: [POST]", http.StatusMethodNotAllowed)
		return
	}

	switch contentType := req.Header.Get("Content-Type"); contentType {
	case "application/json":
		buf := translator.GetBuffer()
		defer translator.PutBuffer(buf)
		if _, err = io.Copy(buf, req.Body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			ddr.params.Logger.Error(err.Error())
			return
		}

		// try parsing as array first, then single record
		var ddLogs []*translator.DatadogLogPayload

		if err = json.Unmarshal(buf.Bytes(), &ddLogs); err != nil {
			// now try parsing as a single record
			var ddLog *translator.DatadogLogPayload
			if err = json.Unmarshal(buf.Bytes(), &ddLog); err != nil {
				http.Error(w, "unable to unmarshal logs", http.StatusBadRequest)
				ddr.params.Logger.Error("unable to unmarshal logs", zap.Error(err))
				return
			}
			ddLogs = append(ddLogs, ddLog)
		}

		plogs := translator.ToPlog(ddLogs)

		logCount = plogs.LogRecordCount()
		err = ddr.nextLogsConsumer.ConsumeLogs(obsCtx, plogs)
		if err != nil {
			errorutil.HTTPError(w, err)
			ddr.params.Logger.Error("log consumer errored out", zap.Error(err))
			return
		}
	default:
		http.Error(w, fmt.Sprintf("unsupported media type '%s', supported: [application/json]", contentType), http.StatusUnsupportedMediaType)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := map[string]any{
		"errors": []string{},
	}
	_ = json.NewEncoder(w).Encode(response)
}

// handleStatsV2 handles incoming stats payloads from datadog agent
// Stats payloads are sent from the DataDog agent at /api/v0.2/stats
func (ddr *datadogReceiver) handleStatsV2(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	metricsCount := 0
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	// Get content encoding from header
	contentEncoding := req.Header.Get("Content-Encoding")

	// Create a reader that handles decompression if needed
	reader, err := createDecompressingReader(req.Body, contentEncoding)
	if err != nil {
		ddr.params.Logger.Error("Error creating decompressing reader", zap.Error(err), zap.String("encoding", contentEncoding))
		http.Error(w, "Error decompressing payload", http.StatusBadRequest)
		return
	}
	defer reader.Close()

	// Decode the StatsPayload (NOT ClientStatsPayload directly)
	// The agent wraps ClientStatsPayload(s) inside a StatsPayload
	statsPayload := &pb.StatsPayload{}
	err = msgp.Decode(reader, statsPayload)
	if err != nil {
		ddr.params.Logger.Error("Error decoding pb.StatsPayload", zap.Error(err))
		http.Error(w, "Error decoding pb.StatsPayload", http.StatusBadRequest)
		return
	}

	// Validate the payload
	if len(statsPayload.Stats) == 0 {
		ddr.params.Logger.Warn("Received empty stats payload")
		_, _ = w.Write([]byte("OK"))
		return
	}

	// Process each ClientStatsPayload within the StatsPayload
	// The agent may send multiple ClientStatsPayload entries in a single request
	for _, clientStats := range statsPayload.Stats {
		// Extract metadata from headers (fallback if not in payload)
		lang := req.Header.Get(header.Lang)
		tracerVersion := req.Header.Get(header.TracerVersion)

		// Translate each client stats payload to metrics
		metrics, translateErr := ddr.statsTranslator.TranslateStats(clientStats, lang, tracerVersion)
		if translateErr != nil {
			err = translateErr
			ddr.params.Logger.Error("Error translating stats", zap.Error(err))
			http.Error(w, "Error translating stats", http.StatusBadRequest)
			return
		}

		metricsCount += metrics.DataPointCount()

		// Send to metrics consumer
		consumeErr := ddr.nextMetricsConsumer.ConsumeMetrics(obsCtx, metrics)
		if consumeErr != nil {
			err = consumeErr
			ddr.params.Logger.Error("Metrics consumer errored out", zap.Error(err))
			errorutil.HTTPError(w, err)
			return
		}
	}

	_, _ = w.Write([]byte("OK"))
}

func (ddr *datadogReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	if req.ContentLength == 0 { // Ping mechanism of Datadog SDK perform http request with empty body when GET /info not implemented.
		http.Error(w, "Fake featuresdiscovery", http.StatusBadRequest) // The response code should be different of 404 to be considered ok by Datadog SDK.
		return
	}
	obsCtx := ddr.tReceiver.StartTracesOp(req.Context())
	var err error
	var spanCount int
	defer func(spanCount *int) {
		ddr.tReceiver.EndTracesOp(obsCtx, "datadog", *spanCount, err)
	}(&spanCount)

	var ddTraces []*pb.TracerPayload
	ddTraces, err = translator.HandleTracesPayload(req)
	if err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusBadRequest)
		ddr.params.Logger.Error("Unable to unmarshal reqs", zap.Error(err))
		return
	}
	for _, ddTrace := range ddTraces {
		otelTraces, err := translator.ToTraces(ddr.params.Logger, ddTrace, req, ddr.traceIDCache)
		if err != nil {
			ddr.params.Logger.Error("Error converting traces", zap.Error(err))
			continue
		}
		spanCount += otelTraces.SpanCount()
		err = ddr.nextTracesConsumer.ConsumeTraces(obsCtx, otelTraces)
		if err != nil {
			errorutil.HTTPError(w, err)
			ddr.params.Logger.Error("Trace consumer errored out", zap.Error(err))
			return
		}
	}

	responseBody := "OK"
	contentType := "text/plain"
	urlSplit := strings.Split(req.RequestURI, "/")
	if len(urlSplit) == 3 {
		// Match the response logic from dd-agent https://github.com/DataDog/datadog-agent/blob/86b2ae24f93941447a5bf0a2b6419caed77e76dd/pkg/trace/api/api.go#L511-L519
		switch version := urlSplit[1]; version {
		case "v0.1", "v0.2", "v0.3":
			// Keep the "OK" response for these versions
		default:
			contentType = "application/json"
			responseBody = "{}"
		}
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write([]byte(responseBody))
}

// handleV1Series handles the v1 series endpoint https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func (ddr *datadogReceiver) handleV1Series(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	buf := translator.GetBuffer()
	defer translator.PutBuffer(buf)
	if _, err = io.Copy(buf, req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		ddr.params.Logger.Error(err.Error())
		return
	}

	seriesList := translator.SeriesList{}
	err = json.Unmarshal(buf.Bytes(), &seriesList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		ddr.params.Logger.Error(err.Error())
		return
	}

	metrics := ddr.metricsTranslator.TranslateSeriesV1(seriesList)
	metricsCount = metrics.DataPointCount()

	err = ddr.nextMetricsConsumer.ConsumeMetrics(obsCtx, metrics)
	if err != nil {
		errorutil.HTTPError(w, err)
		ddr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := map[string]string{
		"status": "ok",
	}
	_ = json.NewEncoder(w).Encode(response)
}

// handleV2Series handles the v2 series endpoint https://docs.datadoghq.com/api/latest/metrics/#submit-metrics
func (ddr *datadogReceiver) handleV2Series(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	series, err := ddr.metricsTranslator.HandleSeriesV2Payload(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		ddr.params.Logger.Error(err.Error())
		return
	}

	metrics := ddr.metricsTranslator.TranslateSeriesV2(series)
	metricsCount = metrics.DataPointCount()

	err = ddr.nextMetricsConsumer.ConsumeMetrics(obsCtx, metrics)
	if err != nil {
		errorutil.HTTPError(w, err)
		ddr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := map[string]any{
		"errors": []string{},
	}
	_ = json.NewEncoder(w).Encode(response)
}

// handleCheckRun handles the service checks endpoint https://docs.datadoghq.com/api/latest/service-checks/
func (ddr *datadogReceiver) handleCheckRun(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	buf := translator.GetBuffer()
	defer translator.PutBuffer(buf)
	if _, err = io.Copy(buf, req.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		ddr.params.Logger.Error(err.Error())
		return
	}

	// try parsing as array first, then single service check
	var services []translator.ServiceCheck

	err = json.Unmarshal(buf.Bytes(), &services)
	if err != nil {
		// now try parsing as a single service check
		var service translator.ServiceCheck
		err = json.Unmarshal(buf.Bytes(), &service)
		if err != nil {
			http.Error(w, "unable to unmarshal service checks", http.StatusBadRequest)
			ddr.params.Logger.Error("unable to unmarshal service checks", zap.Error(err))
			return
		}
		services = append(services, service)
	}

	metrics := ddr.metricsTranslator.TranslateServices(services)
	metricsCount = metrics.DataPointCount()

	err = ddr.nextMetricsConsumer.ConsumeMetrics(obsCtx, metrics)
	if err != nil {
		errorutil.HTTPError(w, err)
		ddr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := map[string]string{
		"status": "ok",
	}
	_ = json.NewEncoder(w).Encode(response)
}

// handleSketches handles sketches, the underlying data structure of distributions https://docs.datadoghq.com/metrics/distributions/
func (ddr *datadogReceiver) handleSketches(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	var ddSketches []gogen.SketchPayload_Sketch
	ddSketches, err = ddr.metricsTranslator.HandleSketchesPayload(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		ddr.params.Logger.Error(err.Error())
		return
	}

	metrics := ddr.metricsTranslator.TranslateSketches(ddSketches)
	metricsCount = metrics.DataPointCount()

	err = ddr.nextMetricsConsumer.ConsumeMetrics(obsCtx, metrics)
	if err != nil {
		errorutil.HTTPError(w, err)
		ddr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("OK"))
}

// handleIntake handles operational calls made by the agent to submit host tags and other metadata to the backend.
func (ddr *datadogReceiver) handleIntake(w http.ResponseWriter, req *http.Request) {
	if ddr.intakeReverseProxy == nil {
		http.Error(w, "intake endpoint not enabled", http.StatusMethodNotAllowed)
	} else {
		ddr.intakeReverseProxy.ServeHTTP(w, req)
	}
}

// handleDistributionPoints handles the distribution points endpoint https://docs.datadoghq.com/api/latest/metrics/#submit-distribution-points
func (ddr *datadogReceiver) handleDistributionPoints(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	err = errors.New("distribution points endpoint not implemented")
	http.Error(w, err.Error(), http.StatusMethodNotAllowed)
	ddr.params.Logger.Warn("metrics consumer errored out", zap.Error(err))
}

// handleStats handles incoming stats payloads.
func (ddr *datadogReceiver) handleStats(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	metricsCount := 0
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	req.Header.Set("Accept", "application/msgpack")
	clientStats := &pb.ClientStatsPayload{}

	err = msgp.Decode(req.Body, clientStats)
	if err != nil {
		ddr.params.Logger.Error("Error decoding pb.ClientStatsPayload", zap.Error(err))
		http.Error(w, "Error decoding pb.ClientStatsPayload", http.StatusBadRequest)
		return
	}

	metrics, err := ddr.statsTranslator.TranslateStats(clientStats, req.Header.Get(header.Lang), req.Header.Get(header.TracerVersion))
	if err != nil {
		ddr.params.Logger.Error("Error translating stats", zap.Error(err))
		http.Error(w, "Error translating stats", http.StatusBadRequest)
		return
	}

	metricsCount = metrics.DataPointCount()

	err = ddr.nextMetricsConsumer.ConsumeMetrics(obsCtx, metrics)
	if err != nil {
		ddr.params.Logger.Error("Metrics consumer errored out", zap.Error(err))
		errorutil.HTTPError(w, err)
		return
	}

	_, _ = w.Write([]byte("OK"))
}

func createIntakeReverseProxyDirector(site, key string) func(*http.Request) {
	host := fmt.Sprintf("api.%s", site)
	query := fmt.Sprintf("api_key=%s", key)
	return func(req *http.Request) {
		req.URL.Scheme = "https"
		req.URL.Host = host
		// we want to use our own API key for all calls
		req.Header.Set("Dd-Api-Key", key)
		// intake puts the API key in the query string as well
		req.URL.RawQuery = query
		// Technically, the JSON body of the `/intake` request contains the API key as well
		// (it's the top-level `apiKey` field of the payload JSON object),
		// but it appears as though the value of that field does not matter,
		// at least when it comes to matching the actual `DD-API-KEY` we set in the header above.
		// So, to avoid a bunch of extra expensive work in the collector, we don't touch the body.
	}
}

// createDecompressingReader creates a reader that handles decompression based on the content encoding.
// Supported encodings: gzip. Returns the original reader if encoding is empty or unsupported.
func createDecompressingReader(body io.ReadCloser, contentEncoding string) (io.ReadCloser, error) {
	switch contentEncoding {
	case "gzip":
		gzReader, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader: %w", err)
		}
		return gzReader, nil
	default:
		return body, nil
	}
}
