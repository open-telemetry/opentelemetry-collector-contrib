// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension"

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

//go:embed html/*
var httpFS embed.FS

type remoteObserverExtension struct {
	config   *Config
	settings extension.CreateSettings
	server   *http.Server
}

func (s *remoteObserverExtension) handleTaps(resp http.ResponseWriter, _ *http.Request) {
	b, err := json.Marshal(s.config.Taps)
	if err != nil {
		s.settings.Logger.Error("error marshaling taps info", zap.Error(err))
		resp.WriteHeader(500)
	} else {
		if _, err = resp.Write(b); err != nil {
			s.settings.Logger.Error("error sending taps info", zap.Error(err))
		}
	}
}

func (s *remoteObserverExtension) Start(_ context.Context, host component.Host) error {
	htmlContent, err := fs.Sub(httpFS, "html")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(htmlContent)))
	mux.HandleFunc("/taps", s.handleTaps)
	s.server, err = s.config.HTTPServerSettings.ToServer(host, s.settings.TelemetrySettings, mux)
	if err != nil {
		return err
	}

	listener, err := s.config.HTTPServerSettings.ToListener()
	if err != nil {
		return err
	}

	go func() {
		err := s.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = s.settings.TelemetrySettings.ReportComponentStatus(component.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

func (s *remoteObserverExtension) Shutdown(_ context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Close()
}
