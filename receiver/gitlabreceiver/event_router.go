// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// eventResult contains the result of processing an event
type eventResult struct {
	Traces  *ptrace.Traces
	Metrics *pmetric.Metrics
}

// eventHandler defines the interface for event handlers
type eventHandler interface {
	// Handle processes the event and returns traces, metrics, or both
	Handle(ctx context.Context, event any) (*eventResult, error)
	// CanHandle returns true if this handler can process the given event type
	CanHandle(event any) bool
}

// eventRouter routes events to appropriate handlers
type eventRouter struct {
	logger   *zap.Logger
	config   *Config
	handlers []eventHandler
}

// newEventRouter creates a new event router with registered handlers
func newEventRouter(logger *zap.Logger, config *Config) *eventRouter {
	router := &eventRouter{
		logger: logger,
		config: config,
		handlers: []eventHandler{
			newPipelineEventHandler(logger, config),
			newJobEventHandler(logger, config),
			// Future event handlers will be added here
		},
	}
	return router
}

// Route routes an event to the appropriate handler
func (er *eventRouter) Route(ctx context.Context, event any, eventType gitlab.EventType) (*eventResult, error) {
	// Find handler that can process this event
	for _, handler := range er.handlers {
		if handler.CanHandle(event) {
			er.logger.Debug("Routing event to handler",
				zap.String("event_type", string(eventType)),
				zap.String("handler", fmt.Sprintf("%T", handler)))
			return handler.Handle(ctx, event)
		}
	}

	// No handler found - log and return no content
	er.logger.Debug("No handler found for event type",
		zap.String("event_type", string(eventType)),
		zap.String("event_type_go", fmt.Sprintf("%T", event)))
	return nil, nil
}
