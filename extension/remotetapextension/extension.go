// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension"

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

//go:embed http/*
var httpFS embed.FS

type remoteTapExtension struct {
	config   *Config
	settings extension.Settings
	server   *http.Server
}

type ComponentID string

// TODO: the extension should implement this interface when available in core
// var _ pdata.Publisher = &remoteTapExtension{}

func (s *remoteTapExtension) Start(ctx context.Context, host component.Host) error {
	htmlContent, err := fs.Sub(httpFS, "html")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(htmlContent)))

	ln, err := s.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", s.config.Endpoint, err)
	}

	s.server, err = s.config.ServerConfig.ToServer(ctx, host, s.settings.TelemetrySettings, mux)
	if err != nil {
		return err
	}

	go func() {
		s.server.Serve(ln)
	}()
	return nil
}

// IsActive returns true when at least one connection is open for the given componentID.
func (s *remoteTapExtension) IsActive(componentID ComponentID) bool {
	return false
}

// PublishMetrics sends metrics for a given componentID.
func (s *remoteTapExtension) PublishMetrics(componentID ComponentID, md pmetric.Metrics) {
	// TODO: do something with the metrics
}

// PublishTraces sends traces for a given componentID.
func (s *remoteTapExtension) PublishTraces(componentID ComponentID, td ptrace.Traces) {
	// TODO: do something with the traces
}

// PublishLogs sends logs for a given componentID.
func (s *remoteTapExtension) PublishLogs(componentID ComponentID, ld plog.Logs) {
	// TODO: do something with the logs
}

func (s *remoteTapExtension) Shutdown(_ context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Close()
}
