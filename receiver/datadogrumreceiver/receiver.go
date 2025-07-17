// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogrumreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/cors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver/internal/translator"
)

type datadogRUMReceiver struct {
	address string
	config  *Config
	params  receiver.Settings

	nextTracesConsumer consumer.Traces
	nextLogsConsumer   consumer.Logs

	server    *http.Server
	lReceiver *receiverhelper.ObsReport

	cancel        context.CancelFunc
}

func newDataDogRUMReceiver(config *Config, params receiver.Settings) (component.Component, error) {
	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	return &datadogRUMReceiver{
		params: params,
		config: config,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		lReceiver: instance,
	}, nil
}

func (ddr *datadogRUMReceiver) Start(ctx context.Context, host component.Host) error {
	ddmux := http.NewServeMux()

	ddmux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if ddr.nextTracesConsumer != nil || ddr.nextLogsConsumer != nil {
		ddmux.HandleFunc("/api/v2/rum", ddr.handleEvent)
	}

	var err error

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://localhost:*", "http://localhost:*"},    // Specify allowed origins
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},                       // Specify allowed methods
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Traceparent"}, // Specify allowed headers
		AllowCredentials: true,                                                     // Allow credentials
	}).Handler(ddmux)

	ddr.server, err = ddr.config.ToServer(
		ctx,
		host,
		ddr.params.TelemetrySettings,
		corsHandler,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := ddr.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	ddr.address = hln.Addr().String()

	ctx, ddr.cancel = context.WithCancel(ctx)

	go func() {
		if err := ddr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting datadog receiver: %w", err)))
			ddr.cancel()
		}
	}()
	return nil
}

func (ddr *datadogRUMReceiver) handleEvent(w http.ResponseWriter, req *http.Request) {
	ddr.params.Logger.Info("Received RUM event")
	obsCtx := ddr.lReceiver.StartTracesOp(req.Context())
	var err error
	var eventCount int
	defer func(eventCount *int) {
		ddr.lReceiver.EndTracesOp(obsCtx, "datadog", *eventCount, err)
	}(&eventCount)

	defer func() {
		_, errs := io.Copy(io.Discard, req.Body)
		err = errors.Join(err, errs, req.Body.Close())
	}()

	buf := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buf, req.Body)
	if err != nil {
		http.Error(w, "Unable to read request body", http.StatusBadRequest)
		ddr.params.Logger.Error("Unable to read request body", zap.Error(err))
		return
	}
	reqBytes := buf.Bytes()

	var jsonEvents []map[string]any
	decoder := json.NewDecoder(buf)
	for {
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			if err.Error() == "EOF" {
				break
			}
			http.Error(w, "Unable to unmarshal reqs", http.StatusBadRequest)
			ddr.params.Logger.Error("Unable to unmarshal reqs", zap.Error(err))
			return
		}
		jsonEvents = append(jsonEvents, event)
	}

	ddr.params.Logger.Info("Request headers", zap.Any("headers", req.Header))
	for _, event := range jsonEvents {
		traceparent := req.Header.Get("traceparent")
		if traceparent == "" && event["_dd"].(map[string]any)["trace_id"] == nil {
			ddr.params.Logger.Info("failed to retrieve W3Ctraceparent or trace_id from header; treating as log instead")
			otelLogs := translator.ToLogs(event, req, reqBytes)
			if ddr.nextLogsConsumer != nil {
				err = ddr.nextLogsConsumer.ConsumeLogs(obsCtx, otelLogs)
			}
		} else {
			ddr.params.Logger.Info("Treating as trace")
			otelTraces := translator.ToTraces(ddr.params.Logger, event, req, reqBytes, traceparent)
			if ddr.nextTracesConsumer != nil {
				err = ddr.nextTracesConsumer.ConsumeTraces(obsCtx, otelTraces)
			}
			eventCount = otelTraces.SpanCount()
		}
		if err != nil {
			http.Error(w, "Log consumer errored out", http.StatusInternalServerError)
			ddr.params.Logger.Error("Log consumer errored out", zap.Error(err))
			return
		}
	}

	_, _ = w.Write([]byte("OK"))
}

func (ddr *datadogRUMReceiver) Shutdown(ctx context.Context) (err error) {
	return ddr.server.Shutdown(ctx)
}
