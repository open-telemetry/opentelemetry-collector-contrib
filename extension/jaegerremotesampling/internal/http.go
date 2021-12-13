// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/jaegertracing/jaeger/cmd/agent/app/configmanager"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

var (
	errMissingClientConfigManager = errors.New("the client config manager has not been provided")
)

var _ component.Component = (*SamplingHTTPServer)(nil)

type SamplingHTTPServer struct {
	telemetry component.TelemetrySettings
	settings  confighttp.HTTPServerSettings
	cfgMgr    configmanager.ClientConfigManager

	mux        *http.ServeMux
	srv        *http.Server
	shutdownWG *sync.WaitGroup
}

func NewHTTP(telemetry component.TelemetrySettings, settings confighttp.HTTPServerSettings, cfgMgr configmanager.ClientConfigManager) (*SamplingHTTPServer, error) {
	if cfgMgr == nil {
		return nil, errMissingClientConfigManager
	}

	srv := &SamplingHTTPServer{
		telemetry: telemetry,
		settings:  settings,
		cfgMgr:    cfgMgr,

		shutdownWG: &sync.WaitGroup{},
	}

	srv.mux = http.NewServeMux()
	// the legacy endpoint
	srv.mux.Handle("/", http.HandlerFunc(srv.samplingStrategyHandler))

	// the new endpoint -- not strictly necessary, as the previous one would match it
	// already, but good to have it explicit here
	srv.mux.Handle("/sampling", http.HandlerFunc(srv.samplingStrategyHandler))

	return srv, nil
}

func (h *SamplingHTTPServer) Start(_ context.Context, host component.Host) error {
	var err error
	h.srv, err = h.settings.ToServer(host, h.telemetry, h.mux)
	if err != nil {
		return err
	}

	h.shutdownWG.Add(1)
	go func() {
		defer h.shutdownWG.Done()

		if err := h.srv.ListenAndServe(); err != http.ErrServerClosed && err != nil {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (h *SamplingHTTPServer) Shutdown(ctx context.Context) error {
	h.shutdownWG.Wait()
	return h.srv.Shutdown(ctx)
}

func (h *SamplingHTTPServer) samplingStrategyHandler(rw http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	if len(svc) == 0 {
		err := errors.New("'service' parameter must be provided")
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.cfgMgr.GetSamplingStrategy(r.Context(), svc)
	if err != nil {
		err = fmt.Errorf("failed to get sampling strategy for service %q: %v", svc, err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		err = fmt.Errorf("cannot convert sampling strategy to JSON: %v", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	rw.Header().Add("Content-Type", "application/json")
	if _, err := rw.Write(jsonBytes); err != nil {
		err = fmt.Errorf("cannot write response to client: %v", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
}
