// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/grpc"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/status"
)

var (
	errNotFound = grpcstatus.Error(codes.NotFound, "Service not found.")

	statusToServingStatusMap = map[component.Status]healthpb.HealthCheckResponse_ServingStatus{
		component.StatusNone:             healthpb.HealthCheckResponse_NOT_SERVING,
		component.StatusStarting:         healthpb.HealthCheckResponse_NOT_SERVING,
		component.StatusOK:               healthpb.HealthCheckResponse_SERVING,
		component.StatusRecoverableError: healthpb.HealthCheckResponse_SERVING,
		component.StatusPermanentError:   healthpb.HealthCheckResponse_SERVING,
		component.StatusFatalError:       healthpb.HealthCheckResponse_NOT_SERVING,
		component.StatusStopping:         healthpb.HealthCheckResponse_NOT_SERVING,
		component.StatusStopped:          healthpb.HealthCheckResponse_NOT_SERVING,
	}
)

func (s *Server) Check(
	_ context.Context,
	req *healthpb.HealthCheckRequest,
) (*healthpb.HealthCheckResponse, error) {
	st, ok := s.aggregator.AggregateStatus(status.Scope(req.Service), status.Concise)
	if !ok {
		return nil, errNotFound
	}

	return &healthpb.HealthCheckResponse{
		Status: s.toServingStatus(st.Event),
	}, nil
}

func (s *Server) toServingStatus(
	ev status.Event,
) healthpb.HealthCheckResponse_ServingStatus {
	if s.componentHealthConfig.IncludeRecoverable &&
		ev.Status() == component.StatusRecoverableError &&
		time.Now().After(ev.Timestamp().Add(s.componentHealthConfig.RecoveryDuration)) {
		return healthpb.HealthCheckResponse_NOT_SERVING
	}

	if s.componentHealthConfig.IncludePermanent && ev.Status() == component.StatusPermanentError {
		return healthpb.HealthCheckResponse_NOT_SERVING
	}

	return statusToServingStatusMap[ev.Status()]
}
