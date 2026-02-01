// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
)

// parseGitlabTime is defined in traces_event_handling.go and reused here

const (
	jobStatusSuccess  = "success"
	jobStatusFailed   = "failed"
	jobStatusCanceled = "canceled"
	jobStatusSkipped  = "skipped"
	jobStatusRunning  = "running"
	jobStatusPending  = "pending"
)

// jobEventHandler handles Job events and converts them to metrics
type jobEventHandler struct {
	logger *zap.Logger
	config *Config
}

// newJobEventHandler creates a new job event handler
func newJobEventHandler(logger *zap.Logger, config *Config) *jobEventHandler {
	return &jobEventHandler{
		logger: logger,
		config: config,
	}
}

// CanHandle returns true if the event is a JobEvent
func (h *jobEventHandler) CanHandle(event interface{}) bool {
	_, ok := event.(*gitlab.JobEvent)
	return ok
}

// Handle processes a Job event and returns metrics
func (h *jobEventHandler) Handle(ctx context.Context, event interface{}) (*eventResult, error) {
	jobEvent, ok := event.(*gitlab.JobEvent)
	if !ok {
		return nil, fmt.Errorf("expected *gitlab.JobEvent, got %T", event)
	}

	// Only process completed jobs
	status := strings.ToLower(jobEvent.BuildStatus)
	if !isCompletedJobStatus(status) {
		h.logger.Debug("Skipping incomplete job",
			zap.String("status", status),
			zap.Int("job_id", jobEvent.BuildID))
		return nil, nil
	}

	// Check if required timestamps are present
	if jobEvent.BuildStartedAt == "" || jobEvent.BuildFinishedAt == "" {
		h.logger.Debug("Job missing timestamps, skipping",
			zap.Int("job_id", jobEvent.BuildID))
		return nil, nil
	}

	// Parse timestamps
	startedAt, err := parseGitlabTime(jobEvent.BuildStartedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse started_at: %w", err)
	}

	finishedAt, err := parseGitlabTime(jobEvent.BuildFinishedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse finished_at: %w", err)
	}

	duration := finishedAt.Sub(startedAt).Seconds()
	queuedDuration := jobEvent.BuildQueuedDuration

	// Create metrics
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes
	setJobResourceAttributes(resourceMetrics.Resource(), jobEvent)

	// Create scope metrics
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scope := scopeMetrics.Scope()
	scope.SetName(metadata.ScopeName)
	// Version is not set as it's not part of the generated metadata

	// Create job duration metric
	if err := createJobDurationMetric(scopeMetrics, jobEvent, duration, status); err != nil {
		return nil, err
	}

	// Create queued duration metric if available
	if queuedDuration > 0 {
		if err := createQueuedDurationMetric(scopeMetrics, jobEvent, queuedDuration); err != nil {
			return nil, err
		}
	}

	return &eventResult{Metrics: &metrics}, nil
}

func isCompletedJobStatus(status string) bool {
	completed := []string{jobStatusSuccess, jobStatusFailed, jobStatusCanceled, jobStatusSkipped}
	for _, s := range completed {
		if status == s {
			return true
		}
	}
	return false
}

func setJobResourceAttributes(resource pcommon.Resource, e *gitlab.JobEvent) {
	attrs := resource.Attributes()
	attrs.PutStr("gitlab.project.id", strconv.Itoa(e.ProjectID))
	attrs.PutStr("gitlab.project.name", e.ProjectName)

	// Service name
	attrs.PutStr(string(conventions.ServiceNameKey), e.ProjectName)
}

func createJobDurationMetric(
	scopeMetrics pmetric.ScopeMetrics,
	e *gitlab.JobEvent,
	duration float64,
	status string,
) error {
	// Create metric
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName("gitlab.job.duration")
	metric.SetUnit("s")
	metric.SetDescription("Job execution duration")

	// Create gauge
	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()

	// Set timestamp
	now := pcommon.NewTimestampFromTime(time.Now())
	dataPoint.SetTimestamp(now)
	dataPoint.SetDoubleValue(duration)

	// Set attributes
	attrs := dataPoint.Attributes()
	attrs.PutStr("gitlab.job.id", strconv.Itoa(e.BuildID))
	attrs.PutStr("gitlab.job.name", e.BuildName)
	attrs.PutStr("gitlab.job.stage", e.BuildStage)
	attrs.PutStr("gitlab.job.status", status)

	// Runner info (Runner is a struct, not a pointer, so check if ID is set)
	if e.Runner.ID > 0 {
		attrs.PutInt("gitlab.job.runner.id", int64(e.Runner.ID))
		if e.Runner.Description != "" {
			attrs.PutStr("gitlab.job.runner.description", e.Runner.Description)
		}
		if len(e.Runner.Tags) > 0 {
			attrs.PutStr("gitlab.job.runner.tags", strings.Join(e.Runner.Tags, ","))
		}
	}

	return nil
}

func createQueuedDurationMetric(
	scopeMetrics pmetric.ScopeMetrics,
	e *gitlab.JobEvent,
	queuedDuration float64,
) error {
	// Create metric
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName("gitlab.job.queued_duration")
	metric.SetUnit("s")
	metric.SetDescription("Job queued duration")

	// Create gauge
	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()

	// Set timestamp
	now := pcommon.NewTimestampFromTime(time.Now())
	dataPoint.SetTimestamp(now)
	dataPoint.SetDoubleValue(queuedDuration)

	// Set attributes
	attrs := dataPoint.Attributes()
	attrs.PutStr("gitlab.job.id", strconv.Itoa(e.BuildID))
	attrs.PutStr("gitlab.job.name", e.BuildName)
	attrs.PutStr("gitlab.job.stage", e.BuildStage)

	return nil
}
