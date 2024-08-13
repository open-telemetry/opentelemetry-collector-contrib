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

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"
)

type datadogReceiver struct {
	address string
	config  *Config
	params  receiver.Settings

	nextTracesConsumer  consumer.Traces
	nextMetricsConsumer consumer.Metrics

	metricsTranslator *translator.MetricsTranslator

	server    *http.Server
	tReceiver *receiverhelper.ObsReport
}

func newDataDogReceiver(config *Config, params receiver.Settings) (component.Component, error) {
	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	return &datadogReceiver{
		params: params,
		config: config,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		tReceiver: instance,
	}, nil
}

func (ddr *datadogReceiver) Start(ctx context.Context, host component.Host) error {
	ddmux := http.NewServeMux()

	ddmux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if ddr.nextTracesConsumer != nil {
		ddmux.HandleFunc("/v0.3/traces", ddr.handleTraces)
		ddmux.HandleFunc("/v0.4/traces", ddr.handleTraces)
		ddmux.HandleFunc("/v0.5/traces", ddr.handleTraces)
		ddmux.HandleFunc("/v0.7/traces", ddr.handleTraces)
		ddmux.HandleFunc("/api/v0.2/traces", ddr.handleTraces)
	}

	if ddr.nextMetricsConsumer != nil {
		ddr.metricsTranslator = translator.NewMetricsTranslator(ddr.params.BuildInfo)

		ddmux.HandleFunc("/api/v1/series", ddr.handleV1Series)
		ddmux.HandleFunc("/api/v2/series", ddr.handleV2Series)
		ddmux.HandleFunc("/api/v1/check_run", ddr.handleCheckRun)
		ddmux.HandleFunc("/api/v1/sketches", ddr.handleSketches)
		ddmux.HandleFunc("/api/beta/sketches", ddr.handleSketches)
		ddmux.HandleFunc("/intake", ddr.handleIntake)
		ddmux.HandleFunc("/api/v1/distribution_points", ddr.handleDistributionPoints)
	}

	var err error
	ddr.server, err = ddr.config.ServerConfig.ToServer(
		ctx,
		host,
		ddr.params.TelemetrySettings,
		ddmux,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := ddr.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	ddr.address = hln.Addr().String()

	go func() {
		if err := ddr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ddr.params.TelemetrySettings.ReportStatus(component.NewFatalErrorEvent(fmt.Errorf("error starting datadog receiver: %w", err)))
		}
	}()
	return nil
}

func (ddr *datadogReceiver) Shutdown(ctx context.Context) (err error) {
	return ddr.server.Shutdown(ctx)
}

func (ddr *datadogReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
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
		otelTraces := translator.ToTraces(ddTrace, req)
		spanCount = otelTraces.SpanCount()
		err = ddr.nextTracesConsumer.ConsumeTraces(obsCtx, otelTraces)
		if err != nil {
			http.Error(w, "Trace consumer errored out", http.StatusInternalServerError)
			ddr.params.Logger.Error("Trace consumer errored out", zap.Error(err))
			return
		}
	}

	_, _ = w.Write([]byte("OK"))

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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		ddr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("OK"))
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		ddr.params.Logger.Error("metrics consumer errored out", zap.Error(err))
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("OK"))
}

// handleCheckRun handles the service checks endpoint https://docs.datadoghq.com/api/latest/service-checks/
func (ddr *datadogReceiver) handleCheckRun(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	err = fmt.Errorf("service checks endpoint not implemented")
	http.Error(w, err.Error(), http.StatusMethodNotAllowed)
	ddr.params.Logger.Warn("metrics consumer errored out", zap.Error(err))
}

// handleSketches handles sketches, the underlying data structure of distributions https://docs.datadoghq.com/metrics/distributions/
func (ddr *datadogReceiver) handleSketches(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	err = fmt.Errorf("sketches endpoint not implemented")
	http.Error(w, err.Error(), http.StatusMethodNotAllowed)
	ddr.params.Logger.Warn("metrics consumer errored out", zap.Error(err))
}

// handleIntake handles operational calls made by the agent to submit host tags and other metadata to the backend.
func (ddr *datadogReceiver) handleIntake(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartMetricsOp(req.Context())
	var err error
	var metricsCount int
	defer func(metricsCount *int) {
		ddr.tReceiver.EndMetricsOp(obsCtx, "datadog", *metricsCount, err)
	}(&metricsCount)

	err = fmt.Errorf("intake endpoint not implemented")
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

	err = fmt.Errorf("distribution points endpoint not implemented")
	http.Error(w, err.Error(), http.StatusMethodNotAllowed)
	ddr.params.Logger.Warn("metrics consumer errored out", zap.Error(err))
}
