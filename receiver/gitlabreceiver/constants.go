// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver"

import (
	"errors"
)

// Pipeline status constants
const (
	// Completed pipeline statuses
	PipelineStatusSuccess  = "success"
	PipelineStatusFailed   = "failed"
	PipelineStatusCanceled = "canceled"
	PipelineStatusSkipped  = "skipped"

	// In-progress pipeline statuses
	PipelineStatusRunning            = "running"
	PipelineStatusPending            = "pending"
	PipelineStatusCreated            = "created"
	PipelineStatusWaitingForResource = "waiting_for_resource"
	PipelineStatusPreparing          = "preparing"
	PipelineStatusScheduled          = "scheduled"
)

// Job status constants
const (
	JobStatusSuccess  = "success"
	JobStatusFailed   = "failed"
	JobStatusCanceled = "canceled"
	JobStatusSkipped  = "skipped"
	JobStatusRunning  = "running"
	JobStatusPending  = "pending"
)

// Custom attribute names (not yet part of semantic conventions)
// These are experimental and subject to change

// VCS (Version Control System) attributes
const (
	AttributeVCSRepositoryVisibility = "vcs.repository.visibility"
	AttributeVCSRepositoryRefDefault = "vcs.repository.ref.default"

	AttributeVCSRefHeadRevisionMessage     = "vcs.ref.head.revision.message"
	AttributeVCSRefHeadRevisionTimestamp   = "vcs.ref.head.revision.timestamp"
	AttributeVCSRefHeadRevisionAuthorName  = "vcs.ref.head.revision.author.name"
	AttributeVCSRefHeadRevisionAuthorEmail = "vcs.ref.head.revision.author.email"
)

// CI/CD attributes
const (
	AttributeCICDPipelineRunActorID   = "cicd.pipeline.run.actor.id"
	AttributeCICDPipelineRunActorName = "cicd.pipeline.run.actor.name"

	AttributeCICDTaskEnvironmentName = "cicd.pipeline.task.run.environment.name"

	AttributeCICDWorkerType   = "cicd.worker.type"
	AttributeCICDWorkerTags   = "cicd.worker.tags"
	AttributeCICDWorkerShared = "cicd.worker.shared"
)

// GitLab-specific attributes (vendor extensions)
const (
	AttributeGitLabProjectNamespace = "gitlab.project.namespace"

	AttributeGitLabPipelineRunActorUsername = "gitlab.pipeline.run.actor.username"

	// Pipeline source attributes
	// Source of the pipeline: https://docs.gitlab.com/ci/jobs/job_rules/#ci_pipeline_source-predefined-variable
	AttributeGitLabPipelineSource              = "gitlab.pipeline.source"
	AttributeGitLabPipelineSourcePipelineID    = "gitlab.pipeline.source.pipeline.id"
	AttributeGitLabPipelineSourcePipelineJobID = "gitlab.pipeline.source.pipeline.job.id"

	AttributeGitLabPipelineSourcePipelineProjectID        = "gitlab.pipeline.source.pipeline.project.id"
	AttributeGitLabPipelineSourcePipelineProjectNamespace = "gitlab.pipeline.source.pipeline.project.namespace"
	AttributeGitLabPipelineSourcePipelineProjectURL       = "gitlab.pipeline.source.pipeline.project.url"

	// Job attributes
	AttributeGitLabJobQueuedDuration = "gitlab.job.queued_duration"
	// Note: failure_reason is now mapped to error.type (semantic convention)
	AttributeGitLabJobAllowFailure = "gitlab.job.allow_failure"

	// Environment attributes
	AttributeGitLabEnvironmentAction         = "gitlab.environment.action"
	AttributeGitLabEnvironmentDeploymentTier = "gitlab.environment.deployment_tier"
)

// HTTP response constants
const (
	HealthyResponse = `{"text": "GitLab receiver webhook is healthy"}`
)

// Error variables for consistent error handling
var (
	// Trace and span ID generation errors
	ErrTraceIDGeneration        = errors.New("failed to generate trace ID")
	ErrPipelineSpanIDGeneration = errors.New("failed to generate pipeline span ID")
	ErrStageSpanIDGeneration    = errors.New("failed to generate stage span ID")
	ErrJobSpanIDGeneration      = errors.New("failed to generate job span ID")

	// Span processing errors
	ErrPipelineSpanProcessing = errors.New("failed to process pipeline span")
	ErrStageSpanProcessing    = errors.New("failed to process stage span")
	ErrJobSpanProcessing      = errors.New("failed to process job span")

	// Timestamp setting errors
	ErrSetPipelineTimestamps = errors.New("failed to set pipeline span timestamps")
	ErrSetStageTimestamps    = errors.New("failed to set stage span timestamps")
	ErrSetJobTimestamps      = errors.New("failed to set job span timestamps")
)
