// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/server/http"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/source"
)

var errMissingStrategyStore = errors.New("the strategy store has not been provided")

var _ component.Component = (*SamplingHTTPServer)(nil)

type SamplingHTTPServer struct {
	telemetry     component.TelemetrySettings
	settings      confighttp.ServerConfig
	strategyStore source.Source

	mux        *http.ServeMux
	srv        *http.Server
	shutdownWG *sync.WaitGroup
}

func NewHTTP(telemetry component.TelemetrySettings, settings confighttp.ServerConfig, strategyStore source.Source) (*SamplingHTTPServer, error) {
	if strategyStore == nil {
		return nil, errMissingStrategyStore
	}

	srv := &SamplingHTTPServer{
		telemetry:     telemetry,
		settings:      settings,
		strategyStore: strategyStore,

		shutdownWG: &sync.WaitGroup{},
	}

	srv.mux = http.NewServeMux()

	// SEE: https://www.jaegertracing.io/docs/1.41/apis/#remote-sampling-configuration-stable
	srv.mux.Handle("/sampling", http.HandlerFunc(srv.samplingStrategyHandler))

	return srv, nil
}

func (h *SamplingHTTPServer) Start(ctx context.Context, host component.Host) error {
	var err error
	h.srv, err = h.settings.ToServer(ctx, host.GetExtensions(), h.telemetry, h.mux)
	if err != nil {
		return err
	}

	var hln net.Listener
	hln, err = h.settings.ToListener(ctx)
	if err != nil {
		return err
	}

	h.shutdownWG.Go(func() {
		if err := h.srv.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	})

	return nil
}

func (h *SamplingHTTPServer) Shutdown(ctx context.Context) error {
	err := h.srv.Shutdown(ctx)
	h.shutdownWG.Wait()
	return err
}

func (h *SamplingHTTPServer) samplingStrategyHandler(rw http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	if svc == "" {
		err := errors.New("'service' parameter must be provided")
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.strategyStore.GetSamplingStrategy(r.Context(), svc)
	if err != nil {
		err = fmt.Errorf("failed to get sampling strategy for service %q: %w", svc, err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		err = fmt.Errorf("cannot convert sampling strategy to JSON: %w", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Add("Content-Type", "application/json")
	if _, err := rw.Write(jsonBytes); err != nil {
		err = fmt.Errorf("cannot write response to client: %w", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
}
