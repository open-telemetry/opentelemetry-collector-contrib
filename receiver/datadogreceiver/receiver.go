// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

type datadogReceiver struct {
	address      string
	config       *Config
	params       receiver.Settings
	nextConsumer consumer.Traces
	server       *http.Server
	tReceiver    *receiverhelper.ObsReport
}

func newDataDogReceiver(config *Config, nextConsumer consumer.Traces, params receiver.Settings) (receiver.Traces, error) {

	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
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

func (ddr *datadogReceiver) Start(ctx context.Context, host component.Host) error {
	ddmux := http.NewServeMux()
	ddmux.HandleFunc("/api/v0.2/traces", ddr.handleV2Traces)
	ddmux.HandleFunc("/v0.2/traces", ddr.handleV2Traces)
	ddmux.HandleFunc("/api/v0.3/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.3/traces", ddr.handleTraces)
	ddmux.HandleFunc("/api/v0.4/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.4/traces", ddr.handleTraces)
	ddmux.HandleFunc("/api/v0.5/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.5/traces", ddr.handleTraces)
	ddmux.HandleFunc("/api/v0.7/traces", ddr.handleTraces)
	ddmux.HandleFunc("/v0.7/traces", ddr.handleTraces)
	ddmux.HandleFunc("/api/v0.2/traces", ddr.handleTraces)

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

func readCloserFromRequest(req *http.Request) (io.ReadCloser, error) {
	rc := struct {
		io.Reader
		io.Closer
	}{
		Reader: req.Body,
		Closer: req.Body,
	}
	if req.Header.Get("Accept-Encoding") == "gzip" {
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		rc.Reader = gz
	}
	return rc, nil
}

func readAndCloseBody(resp http.ResponseWriter, req *http.Request) ([]byte, bool) {
	// Check if the request body is compressed
	var reader io.Reader = req.Body
	if strings.Contains(req.Header.Get("Content-Encoding"), "gzip") {
		// Decompress gzip
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			fmt.Println("err", err)
			//            return
		}
		defer gz.Close()
		reader = gz
	} else if strings.Contains(req.Header.Get("Content-Encoding"), "deflate") {
		// Decompress deflate
		zlibReader, err := zlib.NewReader(req.Body)
		if err != nil {
			fmt.Println("err", err)
			// return
		}
		defer zlibReader.Close()
		reader = zlibReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		fmt.Println("err", err)
		return nil, false
	}
	if err = req.Body.Close(); err != nil {
		fmt.Println("err", err)
		return nil, false
	}
	return body, true
}

func (ddr *datadogReceiver) handleV2Traces(w http.ResponseWriter, req *http.Request) {
	body, err := readAndCloseBody(w, req)
	if !err {
		http.Error(w, "Unable to unmarshal reqs", http.StatusBadRequest)
		ddr.params.Logger.Error("Unable to unmarshal reqs")
		return
	}
	var tracerPayload pb.AgentPayload
	err1 := tracerPayload.UnmarshalVT(body)
	if err1 != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusBadRequest)
		ddr.params.Logger.Error("Unable to unmarshal reqs")
		return
	}
	obsCtx := ddr.tReceiver.StartTracesOp(req.Context())
	tracs := tracerPayload.GetTracerPayloads()
	if len(tracs) > 0 {
		for _, trace := range tracs {
			otelTraces := toTraces(trace, req)
			errs := ddr.nextConsumer.ConsumeTraces(obsCtx, otelTraces)
			if errs != nil {
				http.Error(w, "Trace consumer errored out", http.StatusInternalServerError)
				ddr.params.Logger.Error("Trace consumer errored out")
			} else {
				_, _ = w.Write([]byte("OK"))
			}
		}
	} else {
		_, _ = w.Write([]byte("OK"))
	}
}

func (ddr *datadogReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.tReceiver.StartTracesOp(req.Context())
	var err error
	var spanCount int
	defer func(spanCount *int) {
		ddr.tReceiver.EndTracesOp(obsCtx, "datadog", *spanCount, err)
	}(&spanCount)

	var ddTraces []*pb.TracerPayload
	ddTraces, err = handlePayload(req)
	if err != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusBadRequest)
		ddr.params.Logger.Error("Unable to unmarshal reqs")
		return
	}
	for _, ddTrace := range ddTraces {
		otelTraces := toTraces(ddTrace, req)
		spanCount = otelTraces.SpanCount()
		err = ddr.nextConsumer.ConsumeTraces(obsCtx, otelTraces)
		if err != nil {
			http.Error(w, "Trace consumer errored out", http.StatusInternalServerError)
			ddr.params.Logger.Error("Trace consumer errored out")
			return
		}
	}

	_, _ = w.Write([]byte("OK"))

}
