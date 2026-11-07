// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension"

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension"
)

//go:embed http/*
var httpFS embed.FS

type remoteObserverExtension struct {
	config   *Config
	settings extension.Settings
	server   *http.Server
	stopped  chan struct{}
}

func (s *remoteObserverExtension) Start(ctx context.Context, host component.Host) error {
	htmlContent, err := fs.Sub(httpFS, "http")
	if err != nil {
		return err
	}
	ln, err := s.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", s.config.ServerConfig.NetAddr.Endpoint, err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(htmlContent)))
	server, err := s.config.ServerConfig.ToServer(ctx, host.GetExtensions(), s.settings.TelemetrySettings, mux)
	if err != nil {
		_ = ln.Close()
		return err
	}
	s.server = server

	s.stopped = make(chan struct{})
	go func() {
		defer close(s.stopped)
		err := s.server.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

func (s *remoteObserverExtension) Shutdown(_ context.Context) error {
	if s.server == nil {
		return nil
	}
	err := s.server.Close()
	<-s.stopped
	return err
}
