// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	"github.com/DataDog/datadog-agent/pkg/serializer/marshaler"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
)

var nowFunc = time.Now

// Server provides local metadata server functionality (display the otel_collector payload locally) as well as
// functions to serialize and export this metadata to Datadog backend.
type Server struct {
	// server is used to respond to local metadata requests
	server *http.Server
	// logger is passed from the extension to allow logging
	logger *zap.Logger
	// serializer is a datadog-agent component used to forward payloads to Datadog backend
	serializer agentcomponents.SerializerWithForwarder
	// config contains the httpserver configuration values
	config *Config

	// payload is the metadata to send to Datadog backend
	// TODO: support generic payloads
	payload payload.OtelCollectorPayload

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
	UUID string,
	p payload.OtelCollector,
) *Server {
	oc := payload.OtelCollectorPayload{
		Hostname:  hostname,
		Timestamp: nowFunc().UnixNano(),
		Metadata:  p,
		UUID:      UUID,
	}
	srv := &Server{
		logger:     logger,
		serializer: s,
		config:     config,
		payload:    oc,
	}
	return srv
}

// Start starts the HTTP server and begins sending payloads periodically.
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

// Stop shuts down the HTTP server, pass a context to allow for cancellation.
func (s *Server) Stop(ctx context.Context) {
	if s.server != nil {
		// Create a channel to signal shutdown completion
		shutdownDone := make(chan struct{})

		// Start shutdown in a goroutine
		go func() {
			defer close(shutdownDone) // Ensure channel is always closed
			if err := s.server.Shutdown(ctx); err != nil {
				s.logger.Error("Failed to shutdown HTTP server", zap.Error(err))
			}
		}()

		// Wait for either shutdown completion or context cancellation
		select {
		case <-shutdownDone:
			// Shutdown completed successfully
		case <-ctx.Done():
			// Context was cancelled, log and continue
			s.logger.Warn("Context cancelled while waiting for server shutdown")
			// Wait for the goroutine to finish and close the channel
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

	// Get current time for payload
	s.payload.Timestamp = nowFunc().UnixNano()

	if s.serializer.State() != defaultforwarder.Started {
		return nil, fmt.Errorf("forwarder is not started, extension cannot send payloads to Datadog")
	}

	err := s.serializer.SendMetadata(&s.payload)
	if err != nil {
		return nil, fmt.Errorf("failed to send payload to Datadog: %w", err)
	}

	// Return a copy to avoid race conditions during JSON marshalling
	payloadCopy := s.payload
	return &payloadCopy, nil
}

// HandleMetadata writes the metadata payloads to the response writer and sends them to the Datadog backend
func (s *Server) HandleMetadata(w http.ResponseWriter, r *http.Request) {
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
