// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp"

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/mcp/tools"
)

var _ extension.Extension = (*mcpExtension)(nil)

type mcpExtension struct {
	cfg        *Config
	settings   component.TelemetrySettings
	server     *http.Server
	shutdownWG sync.WaitGroup
}

func newExtension(cfg *Config, telemetry component.TelemetrySettings) *mcpExtension {
	jrse := &mcpExtension{
		cfg:      cfg,
		settings: telemetry,
	}
	return jrse
}

func (mcpe *mcpExtension) Start(ctx context.Context, host component.Host) error {
	var err error

	// Create a new MCP server
	s := server.NewMCPServer(
		"otel-mcp-server",
		"0.0.1",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Get all tools from the tools package
	allTools, err := tools.GetAllTools()
	if err != nil {
		return err
	}

	// Register all tools with the server
	for _, tool := range allTools { //nolint:gocritic
		s.AddTool(tool.Tool, tool.Handler)
	}

	mcpe.server, err = mcpe.cfg.ToServer(ctx, host.GetExtensions(), mcpe.settings, server.NewStreamableHTTPServer(s))
	if err != nil {
		return err
	}

	mcpe.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", mcpe.cfg.NetAddr.Endpoint))
	var listener net.Listener
	if listener, err = mcpe.cfg.ToListener(ctx); err != nil {
		return err
	}

	mcpe.shutdownWG.Go(func() {
		if err := mcpe.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	})

	return nil
}

func (mcpe *mcpExtension) Shutdown(ctx context.Context) error {
	if mcpe.server != nil {
		if err := mcpe.server.Shutdown(ctx); err != nil {
			mcpe.settings.Logger.Error("error while shutting down the HTTP server", zap.Error(err))
		}
	}

	return nil
}
