// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
)

type datadogReceiver struct {
	config       *Config
	params       receiver.CreateSettings
	nextConsumer consumer.Traces
	server       *http.Server
	tReceiver    *obsreport.Receiver
}

func newDataDogReceiver(config *Config, nextConsumer consumer.Traces, params receiver.CreateSettings) (receiver.Traces, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	instance, err := obsreport.NewReceiver(obsreport.ReceiverSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	return &datadogReceiver{
		params:       params,
		config:       config,
		nextConsumer: nextConsumer,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		tReceiver: instance,
	}, nil
}

func (ddr *datadogReceiver) Start(_ context.Context, host component.Host) error {
	ddmux := http.NewServeMux()
	ddmux.HandleFunc("/v0.3/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.4/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.5/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.7/traces", ddr.handleTraces)

	var err error
	ddr.server, err = ddr.config.HTTPServerSettings.ToServer(
		host,
		ddr.params.TelemetrySettings,
		ddmux,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := ddr.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	go func() {
		if err := ddr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			host.ReportFatalError(fmt.Errorf("error starting datadog receiver: %w", err))
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
	var ddTraces *pb.TracerPayload

	ddTraces, err = handlePayload(req)
	if err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusInternalServerError)
		ddr.params.Logger.Error("Unable to unmarshal reqs")
	}

	otelTraces := toTraces(ddTraces, req)
	spanCount = otelTraces.SpanCount()
	err = ddr.nextConsumer.ConsumeTraces(obsCtx, otelTraces)
	if err != nil {
		http.Error(w, "Trace consumer errored out", http.StatusInternalServerError)
		ddr.params.Logger.Error("Trace consumer errored out")
	} else {
		_, _ = w.Write([]byte("OK"))
	}
}
