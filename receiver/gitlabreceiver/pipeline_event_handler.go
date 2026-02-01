// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// pipelineEventHandler handles Pipeline events and converts them to traces
type pipelineEventHandler struct {
	logger *zap.Logger
	config *Config
}

// newPipelineEventHandler creates a new pipeline event handler
func newPipelineEventHandler(logger *zap.Logger, config *Config) *pipelineEventHandler {
	return &pipelineEventHandler{
		logger: logger,
		config: config,
	}
}

// CanHandle returns true if the event is a PipelineEvent
func (h *pipelineEventHandler) CanHandle(event interface{}) bool {
	_, ok := event.(*gitlab.PipelineEvent)
	return ok
}

// Handle processes a Pipeline event and returns traces
func (h *pipelineEventHandler) Handle(ctx context.Context, event interface{}) (*eventResult, error) {
	pipelineEvent, ok := event.(*gitlab.PipelineEvent)
	if !ok {
		return nil, fmt.Errorf("expected *gitlab.PipelineEvent, got %T", event)
	}

	// Check if the finishedAt timestamp is present, which is required for traceID generation
	if pipelineEvent.ObjectAttributes.FinishedAt == "" {
		h.logger.Debug("pipeline missing finishedAt timestamp, skipping...",
			zap.String("status", pipelineEvent.ObjectAttributes.Status))
		return nil, nil
	}

	// Process the pipeline based on its status
	status := strings.ToLower(pipelineEvent.ObjectAttributes.Status)
	switch status {
	case PipelineStatusRunning, PipelineStatusPending, PipelineStatusCreated, PipelineStatusWaitingForResource, PipelineStatusPreparing, PipelineStatusScheduled:
		h.logger.Debug("pipeline not complete, skipping...",
			zap.String("status", pipelineEvent.ObjectAttributes.Status))
		return nil, nil
	case PipelineStatusSuccess, PipelineStatusFailed, PipelineStatusCanceled, PipelineStatusSkipped:
		// above statuses are indicators of a completed pipeline, so we process them
		break
	default:
		h.logger.Warn("unknown pipeline status, skipping...",
			zap.String("status", pipelineEvent.ObjectAttributes.Status))
		return nil, nil
	}

	// Validate pipeline event
	if err := validatePipelineEvent(pipelineEvent); err != nil {
		return nil, fmt.Errorf("invalid pipeline event: %w", err)
	}

	// Create receiver instance for handling (we need access to receiver methods)
	// For now, we'll use a helper that doesn't require the full receiver
	traces, err := handlePipelineEvent(pipelineEvent, h.logger, h.config)
	if err != nil {
		return nil, fmt.Errorf("failed to handle pipeline event: %w", err)
	}

	return &eventResult{Traces: &traces}, nil
}

// validatePipelineEvent validates critical webhook event fields for trace generation
func validatePipelineEvent(e *gitlab.PipelineEvent) error {
	if e.ObjectAttributes.ID == 0 {
		return fmt.Errorf("%w: pipeline ID", errMissingRequiredField)
	}

	if e.ObjectAttributes.CreatedAt == "" {
		return fmt.Errorf("%w: pipeline created_at", errMissingRequiredField)
	}

	if e.ObjectAttributes.Ref == "" {
		return fmt.Errorf("%w: pipeline ref", errMissingRequiredField)
	}

	if e.ObjectAttributes.SHA == "" {
		return fmt.Errorf("%w: pipeline SHA", errMissingRequiredField)
	}

	if e.Project.ID == 0 {
		return fmt.Errorf("%w: project ID", errMissingRequiredField)
	}

	if e.Project.PathWithNamespace == "" {
		return fmt.Errorf("%w: project path_with_namespace", errMissingRequiredField)
	}

	if e.Commit.ID == "" {
		return fmt.Errorf("%w: commit ID", errMissingRequiredField)
	}

	return nil
}

// handlePipelineEvent processes a pipeline event and returns traces
// This uses the existing handlePipeline logic from traces_event_handling.go
func handlePipelineEvent(e *gitlab.PipelineEvent, logger *zap.Logger, cfg *Config) (ptrace.Traces, error) {
	// Create a gitlabTracesReceiver instance to access the existing pipeline handling methods
	gtr := &gitlabTracesReceiver{
		logger: logger,
		cfg:    cfg,
	}
	return gtr.handlePipeline(e)
}
