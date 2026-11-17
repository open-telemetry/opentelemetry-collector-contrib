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
	telemetry         *instrumentationTelemetry
	telemetryOnly     bool
	maxRCRequestBytes int64
	config            *Config
	settings          extension.Settings
	newService        func(context.Context, component.Host) (remoteConfigService, error)
	cancelBackend     context.CancelFunc
	service           remoteConfigService
	server            *http.Server
	listener          net.Listener
	done              chan struct{}
}

func (e *remoteConfigExtension) Start(ctx context.Context, host component.Host) error {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeout := e.config.RequestTimeout
		if r.URL.Path == telemetryPath && e.telemetry != nil {
			timeout = e.telemetry.config.Timeout
		}
		http.TimeoutHandler(http.HandlerFunc(e.serveHTTP), timeout, "SDK request timed out").ServeHTTP(w, r)
	})
	server, err := e.config.ServerConfig.ToServer(ctx, host.GetExtensions(), e.settings.TelemetrySettings, handler)
	if err != nil {
		return err
	}
	if e.telemetry != nil {
		// RC retains Collector decoding. Telemetry preserves compressed bytes
		// under the same TLS, authentication and request-limit policy.
		opaqueConfig := e.config.ServerConfig
		opaqueConfig.CompressionAlgorithms = []string{}
		opaque, opaqueErr := opaqueConfig.ToServer(ctx, host.GetExtensions(), e.settings.TelemetrySettings, handler)
		if opaqueErr != nil {
			return opaqueErr
		}
		normal := server.Handler
		server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == telemetryPath {
				opaque.Handler.ServeHTTP(w, r)
				return
			}
			normal.ServeHTTP(w, r)
		})
	}
	listener, err := e.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	backendContext, cancel := context.WithCancel(context.Background())
	cleanup := func() {
		cancel()
		if e.telemetry != nil {
			e.telemetry.close()
		}
		_ = listener.Close()
	}
	if e.telemetry != nil {
		if err := e.telemetry.start(backendContext, host, e.settings.TelemetrySettings); err != nil {
			cleanup()
			return err
		}
	}
	if !e.telemetryOnly {
		service, err := e.newService(backendContext, host)
		if err != nil {
			cleanup()
			return err
		}
		e.service = service
		service.Start()
	}
	e.cancelBackend = cancel
	e.server = server
	e.listener = listener
	e.done = make(chan struct{})
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
	var serviceErr error
	if e.service != nil {
		serviceErr = e.service.Stop()
	}
	if e.telemetry != nil {
		e.telemetry.close()
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
		if e.telemetryOnly {
			http.NotFound(w, r)
			return
		}
		method = http.MethodPost
	case telemetryPath:
		if e.telemetry == nil {
			http.NotFound(w, r)
			return
		}
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
		endpoints := []string{}
		if !e.telemetryOnly {
			endpoints = append(endpoints, "/v0.7/config")
		}
		if e.telemetry != nil {
			endpoints = append(endpoints, "/telemetry/proxy/")
		}
		writeJSON(w, map[string]any{"version": version.AgentVersion, "endpoints": endpoints})
		return
	}
	if r.URL.Path == telemetryPath {
		e.telemetry.serveHTTP(w, r)
		return
	}
	var request pbgo.ClientGetConfigsRequest
	limit := e.maxRCRequestBytes
	if limit == 0 {
		limit = e.config.ServerConfig.MaxRequestBodySize
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
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
