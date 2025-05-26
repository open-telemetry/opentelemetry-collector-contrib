// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/DataDog/agent-payload/v5/gogen"
	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator/header"
)

type datadogReceiver struct {
	address string
	config  *Config
	params  receiver.Settings

	nextTracesConsumer  consumer.Traces
	nextMetricsConsumer consumer.Metrics

	metricsTranslator *translator.MetricsTranslator
	statsTranslator   *translator.StatsTranslator

	server    *http.Server
	tReceiver *receiverhelper.ObsReport

	traceIDCache *simplelru.LRU[uint64, pcommon.TraceID]
}

// Endpoint specifies an API endpoint definition.
type Endpoint struct {
	// Pattern specifies the API pattern, as registered by the HTTP handler.
	Pattern string

	// Handler specifies the http.Handler for this endpoint.
	Handler func(http.ResponseWriter, *http.Request)
}

// getEndpoints specifies the list of endpoints registered for the trace-agent API.
func (ddr *datadogReceiver) getEndpoints() []Endpoint {
	endpoints := []Endpoint{
		{
			Pattern: "/",
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
	}

	if ddr.nextTracesConsumer != nil {
		endpoints = append(endpoints, []Endpoint{
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
		endpoints = append(endpoints, []Endpoint{
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
				Pattern: "/api/v1/distribution_points",
				Handler: ddr.handleDistributionPoints,
			},
			{
				Pattern: "/v0.6/stats",
				Handler: ddr.handleStats,
			},
		}...)
	}

	infoResponse, _ := ddr.buildInfoResponse(endpoints)

	endpoints = append(endpoints, Endpoint{
		Pattern: "/info",
		Handler: func(w http.ResponseWriter, r *http.Request) { ddr.handleInfo(w, r, infoResponse) },
	})

	return endpoints
}

func newDataDogReceiver(config *Config, params receiver.Settings) (component.Component, error) {
	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	var cache *simplelru.LRU[uint64, pcommon.TraceID]
	if FullTraceIDFeatureGate.IsEnabled() {
		cache, err = simplelru.NewLRU[uint64, pcommon.TraceID](config.TraceIDCacheSize, func(k uint64, _ pcommon.TraceID) {
			params.Logger.Debug("evicting datadog trace id from cache", zap.Uint64("id", k))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create traceID cache: %w", err)
		}
	}

	return &datadogReceiver{
		params: params,
		config: config,
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
		host,
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

func (ddr *datadogReceiver) buildInfoResponse(endpoints []Endpoint) ([]byte, error) {
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
		spanCount = otelTraces.SpanCount()
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

	var services []translator.ServiceCheck

	err = json.Unmarshal(buf.Bytes(), &services)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		ddr.params.Logger.Error(err.Error())
		return
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
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	err = errors.New("intake endpoint not implemented")
	http.Error(w, err.Error(), http.StatusMethodNotAllowed)
	ddr.params.Logger.Warn("metrics consumer errored out", zap.Error(err))
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
