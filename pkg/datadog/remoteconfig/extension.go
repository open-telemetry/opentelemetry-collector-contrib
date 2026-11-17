// Copyright The OpenTelemetry Authors
// Copyright 2026-present Datadog, Inc.
// SPDX-License-Identifier: Apache-2.0

package remoteconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/remoteconfig"

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"

	pbgo "github.com/DataDog/datadog-agent/pkg/proto/pbgo/core"
	"github.com/DataDog/datadog-agent/pkg/trace/traceutil/normalize"
	"github.com/DataDog/datadog-agent/pkg/version"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type remoteConfigExtension struct {
	closeBackend  func()
	config        *Config
	settings      extension.Settings
	newService    func(context.Context, component.Host) (remoteConfigService, error)
	cancelBackend context.CancelFunc
	service       remoteConfigService
	server        *http.Server
	listener      net.Listener
	done          chan struct{}
}

func (e *remoteConfigExtension) Start(ctx context.Context, host component.Host) error {
	handler := http.TimeoutHandler(http.HandlerFunc(e.serveHTTP), e.config.RequestTimeout, "Remote Configuration request timed out")
	server, err := e.config.ServerConfig.ToServer(ctx, host.GetExtensions(), e.settings.TelemetrySettings, handler)
	if err != nil {
		return err
	}
	listener, err := e.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	backendContext, cancel := context.WithCancel(context.Background())
	service, err := e.newService(backendContext, host)
	if err != nil {
		cancel()
		if e.closeBackend != nil {
			e.closeBackend()
		}
		_ = listener.Close()
		return err
	}
	e.service = service
	e.cancelBackend = cancel
	e.server = server
	e.listener = listener
	e.done = make(chan struct{})
	service.Start()
	go func() {
		defer close(e.done)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

func (e *remoteConfigExtension) Shutdown(ctx context.Context) error {
	if e.server == nil {
		return nil
	}
	e.cancelBackend()
	err := e.server.Shutdown(ctx)
	if err != nil {
		_ = e.server.Close()
	}
	<-e.done
	serviceErr := e.service.Stop()
	if e.closeBackend != nil {
		e.closeBackend()
	}
	return errors.Join(err, serviceErr)
}

func (e *remoteConfigExtension) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		http.Error(w, "query parameters are not supported", http.StatusBadRequest)
		return
	}
	var method string
	switch r.URL.Path {
	case "/info":
		method = http.MethodGet
	case "/v0.7/config":
		method = http.MethodPost
	default:
		http.NotFound(w, r)
		return
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/info" {
		writeJSON(w, map[string]any{"version": version.AgentVersion, "endpoints": []string{"/v0.7/config"}})
		return
	}
	var request pbgo.ClientGetConfigsRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, e.config.ServerConfig.MaxRequestBodySize))
	if err := decoder.Decode(&request); err != nil {
		writeDecodeError(w, err)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeDecodeError(w, err)
		return
	}
	if request.Client == nil || !request.Client.IsTracer || request.Client.ClientTracer == nil || request.Client.IsAgent || request.Client.IsUpdater {
		http.Error(w, "a tracer client is required", http.StatusBadRequest)
		return
	}
	tracer := request.Client.ClientTracer
	tracer.Service, _ = normalize.NormalizeService(tracer.Service, tracer.Language)
	tracer.Env = normalize.NormalizeTagValue(tracer.Env)
	ctx, cancel := context.WithTimeout(r.Context(), e.config.RequestTimeout)
	defer cancel()
	response, err := e.service.ClientGetConfigs(ctx, &request)
	if err != nil {
		code := http.StatusInternalServerError
		switch status.Code(err) {
		case codes.InvalidArgument:
			code = http.StatusBadRequest
		case codes.DeadlineExceeded:
			code = http.StatusGatewayTimeout
		case codes.Unavailable:
			code = http.StatusServiceUnavailable
		}
		http.Error(w, http.StatusText(code), code)
		return
	}
	writeJSON(w, response)
}

func writeDecodeError(w http.ResponseWriter, err error) {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid Remote Configuration request", http.StatusBadRequest)
}

func writeJSON(w http.ResponseWriter, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "could not encode Remote Configuration response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}
