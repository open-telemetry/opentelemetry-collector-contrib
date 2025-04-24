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
	"github.com/DataDog/datadog-agent/pkg/serializer"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/componentchecker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
)

var nowFunc = time.Now

// defaultForwarderInterface is wrapper for methods in datadog-agent DefaultForwarder struct
type defaultForwarderInterface interface {
	defaultforwarder.Forwarder
	Start() error
	State() uint32
	Stop()
}

// Server provides local metadata server functionality (display the otel_collector payload locally) as well as
// functions to serialize and export this metadata to Datadog backend.
// Specifically, making a request to the local endpoint, configured in config.go, will both display the
// otel_collector payload as well as submit it to the Datadog backend.
// Server provides local metadata server functionality (display the otel_collector payload locally) as well as
// functions to serialize and export this metadata to Datadog backend.
// Specifically, making a request to the local endpoint, configured in config.go, will both display the
// otel_collector payload as well as submit it to the Datadog backend.
type Server struct {
	server               *http.Server
	logger               *zap.Logger
	serializer           serializer.MetricSerializer
	forwarder            defaultForwarderInterface
	cancel               context.CancelFunc
	config               *Config
	hostnameSource       string
	hostname             string
	uuid                 string
	componentStatus      *map[string]any
	moduleInfo           *service.ModuleInfos
	collectorConfig      *confmap.Conf
	otelCollectorPayload *payload.OtelCollector
}

// NewServer creates a new HTTP server instance
func NewServer(
	logger *zap.Logger,
	config *Config,
	s serializer.MetricSerializer,
	f defaultForwarderInterface,
	hs string,
	hn string,
	UUID string,
	componentstatus *map[string]any,
	moduleinfo *service.ModuleInfos,
	collectorconfig *confmap.Conf,
	collectorpayload *payload.OtelCollector,
) *Server {
	srv := &Server{
		logger:               logger,
		serializer:           s,
		forwarder:            f,
		config:               config,
		hostnameSource:       hs,
		hostname:             hn,
		uuid:                 UUID,
		componentStatus:      componentstatus,
		moduleInfo:           moduleinfo,
		collectorConfig:      collectorconfig,
		otelCollectorPayload: collectorpayload,
	}
	return srv
}

// Start starts the HTTP server and begins sending payloads periodically
func (s *Server) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Create a mux to handle only the specific path
	mux := http.NewServeMux()
	mux.HandleFunc(s.config.Path, s.HandleMetadata)

	s.server = &http.Server{
		Addr:         s.config.Endpoint,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		BaseContext:  func(net.Listener) context.Context { return ctx },
	}

	// Start HTTP server
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Monitor context cancellation
	go func() {
		<-ctx.Done()
		if err := s.server.Shutdown(context.Background()); err != nil {
			s.logger.Error("Error during server shutdown", zap.Error(err))
		}
	}()

	s.logger.Info("HTTP Server started at " + s.config.Endpoint + s.config.Path)
}

// Stop shuts down the HTTP server
func (s *Server) Stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("Failed to shutdown HTTP server", zap.Error(err))
		}
	}
	if s.cancel != nil {
		s.cancel()
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

	healthStatus := componentchecker.DataToFlattenedJSONString(*s.componentStatus)
	otelCollectorPayload.HealthStatus = healthStatus

	oc := payload.OtelCollectorPayload{
		Hostname:  s.hostname,
		Timestamp: nowFunc().UnixNano(),
		Metadata:  otelCollectorPayload,
		UUID:      s.uuid,
	}

	// Use datadog-agent serializer to send these payloads
	if s.forwarder.State() != defaultforwarder.Started {
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
