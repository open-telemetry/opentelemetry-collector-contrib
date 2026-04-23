// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/pkg/serializer/marshaler"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
)

var nowFunc = time.Now

// Server provides local metadata server functionality (display the otel_collector payload locally) as well as
// functions to serialize and export this metadata to Datadog backend.
type Server struct {
	// serverConfig is used to create the HTTP server via confighttp
	serverConfig *confighttp.ServerConfig
	// handler is the HTTP handler for the server
	handler http.Handler
	// listenClose is used to shut down the server
	listenClose func(ctx context.Context) error
	// logger is passed from the extension to allow logging
	logger *zap.Logger
	// telemetrySettings holds the telemetry settings for the server
	telemetrySettings component.TelemetrySettings
	// serializer is a datadog-agent component used to forward payloads to Datadog backend
	serializer agentcomponents.SerializerWithForwarder
	// config contains the httpserver configuration values
	config *Config

	// payload is the metadata to send to Datadog backend
	payload marshaler.JSONMarshaler

	// mu protects concurrent access to the serializer
	// Note: Only protects serializer operations, not field reads since they're set once during initialization
	mu sync.Mutex
}

// NewServer creates a new HTTP server instance.
// It should be called after NotifyConfig has received full configuration.
// TODO: support generic payloads
func NewServer(
	logger *zap.Logger,
	s agentcomponents.SerializerWithForwarder,
	config *Config,
	hostname string,
	uuid string,
	p payload.OtelCollector,
	telemetrySettings component.TelemetrySettings,
) *Server {
	// Create payload but don't add timestamp, that will happen in SendPayload
	oc := &payload.OtelCollectorPayload{
		Hostname: hostname,
		Metadata: p,
		UUID:     uuid,
	}

	srv := &Server{
		logger:            logger,
		telemetrySettings: telemetrySettings,
		serializer:        s,
		config:            config,
		serverConfig:      &config.ServerConfig,
		payload:           oc, // store as interface
	}

	mux := http.NewServeMux()
	mux.HandleFunc(config.Path, srv.HandleMetadata)
	srv.handler = mux

	return srv
}

// Start starts the HTTP server and begins sending payloads periodically.
func (s *Server) Start(ctx context.Context, host component.Host) error {
	server, err := s.serverConfig.ToServer(
		ctx,
		host.GetExtensions(),
		s.telemetrySettings,
		s.handler,
	)
	if err != nil {
		return err
	}

	listener, err := s.serverConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	s.listenClose = server.Shutdown

	// Start HTTP server
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	s.logger.Info("HTTP Server started at " + s.config.NetAddr.Endpoint + s.config.Path)
	return nil
}

// Stop shuts down the HTTP server, pass a context to allow for cancellation.
func (s *Server) Stop(ctx context.Context) {
	if s.listenClose != nil {
		shutdownDone := make(chan struct{})

		go func() {
			defer close(shutdownDone) // Ensure channel is always closed
			if err := s.listenClose(ctx); err != nil {
				s.logger.Error("Failed to shutdown HTTP server", zap.Error(err))
			}
		}()

		select {
		case <-shutdownDone:
		case <-ctx.Done():
			s.logger.Warn("Context cancelled while waiting for server shutdown")
			<-shutdownDone
		}
	}
}

// SendPayload prepares and sends the fleet automation payloads using Server's handlerDeps
// TODO: support generic payloads
func (s *Server) SendPayload() (marshaler.JSONMarshaler, error) {
	// Use datadog-agent serializer to send these payloads
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clone the payload to avoid data races
	var payloadCopy marshaler.JSONMarshaler
	if oc, ok := s.payload.(*payload.OtelCollectorPayload); ok {
		tmp := *oc // shallow copy is sufficient since fields are value types or slices (which are not mutated)
		tmp.Timestamp = nowFunc().UnixNano()
		payloadCopy = &tmp
	} else {
		payloadCopy = s.payload
	}

	if s.serializer.State() != defaultforwarder.Started {
		return nil, errors.New("forwarder is not started, extension cannot send payloads to Datadog")
	}

	err := s.serializer.SendMetadata(payloadCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to send payload to Datadog: %w", err)
	}

	return payloadCopy, nil
}

// HandleMetadata writes the metadata payloads to the response writer and sends them to the Datadog backend
func (s *Server) HandleMetadata(w http.ResponseWriter, _ *http.Request) {
	fullPayload, err := s.SendPayload()
	if err != nil {
		s.logger.Error("Failed to prepare and send fleet automation payload", zap.Error(err))
		if w != nil {
			http.Error(w, "Failed to prepare and send fleet automation payload", http.StatusInternalServerError)
		}
		return
	}

	// Marshal the combined payload to JSON
	// Note: fullPayload is already thread-safe since SendPayload returned a marshaler interface
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
