// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/componentchecker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
)

var nowFunc = time.Now

// Server provides local metadata server functionality (display the otel_collector payload locally) as well as
// functions to serialize and export this metadata to Datadog backend.
type Server struct {
	server     *http.Server
	logger     *zap.Logger
	serializer agentcomponents.SerializerWithForwarder
	// config is the httpserver configuration values
	config *Config

	// hostname and uuid are required to send the payload to Datadog
	hostname string
	uuid     string

	// moduleInfo is a list of configured modules available to collector
	moduleInfo *service.ModuleInfos
	// collectorConfig is the full configuration for the collector, usually received by NotifyConfig interface
	collectorConfig *confmap.Conf
	// otelCollectorPayload is the collector metadata required to send to Datadog backend
	otelCollectorPayload *payload.OtelCollector
}

// NewServer creates a new HTTP server instance
func NewServer(
	logger *zap.Logger,
	config *Config,
	s agentcomponents.SerializerWithForwarder,
	hs string,
	hn string,
	UUID string,
	componentstatus map[string]any,
	moduleinfo *service.ModuleInfos,
	collectorconfig *confmap.Conf,
	collectorpayload *payload.OtelCollector,
) *Server {
	srv := &Server{
		logger:               logger,
		serializer:           s,
		config:               config,
		hostname:             hn,
		uuid:                 UUID,
		moduleInfo:           moduleinfo,
		collectorConfig:      collectorconfig,
		otelCollectorPayload: collectorpayload,
	}
	return srv
}

// Start starts the HTTP server and begins sending payloads periodically
func (s *Server) Start() {
	// Create a mux to handle only the specific path
	mux := http.NewServeMux()
	mux.HandleFunc(s.config.Path, s.HandleMetadata)

	s.server = &http.Server{
		Addr:         s.config.Endpoint,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		BaseContext:  func(net.Listener) context.Context { return context.Background() },
	}

	// Start HTTP server
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	s.logger.Info("HTTP Server started at " + s.config.Endpoint + s.config.Path)
}

// Stop shuts down the HTTP server
func (s *Server) Stop(ctx context.Context) {
	if s.server != nil {
		// Create a channel to signal shutdown completion
		shutdownDone := make(chan struct{})

		// Start shutdown in a goroutine
		go func() {
			if err := s.server.Shutdown(ctx); err != nil {
				s.logger.Error("Failed to shutdown HTTP server", zap.Error(err))
			}
			close(shutdownDone)
		}()

		// Wait for either shutdown completion or context cancellation
		select {
		case <-shutdownDone:
			// Shutdown completed successfully
		case <-ctx.Done():
			// Context was cancelled, log and continue
			s.logger.Warn("Context cancelled while waiting for server shutdown")
			close(shutdownDone)
		}
	}
}

// PrepareAndSendFleetAutomationPayloads prepares and sends the fleet automation payloads using Server's handlerDeps
func (s *Server) PrepareAndSendFleetAutomationPayloads() (*payload.OtelCollector, error) {
	logger := s.logger
	otelCollectorPayload := *s.otelCollectorPayload

	moduleInfoJSON, err := componentchecker.PopulateFullComponentsJSON(*s.moduleInfo, s.collectorConfig)
	if err != nil {
		logger.Error("Failed to populate full components JSON", zap.Error(err))
	}
	activeComponents, err := componentchecker.PopulateActiveComponents(s.collectorConfig, moduleInfoJSON)
	if err != nil {
		logger.Error("Failed to populate active components JSON", zap.Error(err))
	}
	// add remaining data to otelCollectorPayload
	otelCollectorPayload.FullComponents = moduleInfoJSON.GetFullComponentsList()
	if activeComponents != nil {
		otelCollectorPayload.ActiveComponents = *activeComponents
	}

	oc := payload.OtelCollectorPayload{
		Hostname:  s.hostname,
		Timestamp: nowFunc().UnixNano(),
		Metadata:  otelCollectorPayload,
		UUID:      s.uuid,
	}

	// Use datadog-agent serializer to send these payloads
	if s.serializer.State() != defaultforwarder.Started {
		return nil, fmt.Errorf("forwarder is not started, extension cannot send payloads to Datadog: %w", err)
	}
	err = s.serializer.SendMetadata(&oc)
	if err != nil {
		return nil, fmt.Errorf("failed to send otel_collector payload: %w", err)
	}
	return &otelCollectorPayload, nil
}

// HandleMetadata writes the metadata payloads to the response writer and sends them to the Datadog backend
func (s *Server) HandleMetadata(w http.ResponseWriter, r *http.Request) {
	fullPayload, err := s.PrepareAndSendFleetAutomationPayloads()
	if err != nil {
		s.logger.Error("Failed to prepare and send fleet automation payload", zap.Error(err))
		if w != nil {
			http.Error(w, "Failed to prepare and send fleet automation payload", http.StatusInternalServerError)
		}
		return
	}

	// Marshal the combined payload to JSON
	jsonData, err := json.MarshalIndent(fullPayload, "", "  ")
	if err != nil {
		s.logger.Error("Failed to marshal collector payload for local http response", zap.Error(err))
		if w != nil {
			http.Error(w, "Failed to marshal collector payload", http.StatusInternalServerError)
		}
		return
	}

	if w != nil {
		// Write the JSON response
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(jsonData)
		if err != nil {
			s.logger.Error("Failed to write response to local metadata request", zap.Error(err))
		}
	}
}
