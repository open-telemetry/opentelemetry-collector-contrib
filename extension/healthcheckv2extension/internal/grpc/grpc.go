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
	errNotFound     = grpcstatus.Error(codes.NotFound, "Service not found.")
	errShuttingDown = grpcstatus.Error(codes.Canceled, "Server shutting down.")
	errStreamSend   = grpcstatus.Error(codes.Canceled, "Error sending; stream terminated.")
	errStreamEnded  = grpcstatus.Error(codes.Canceled, "Stream has ended.")

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

func (s *Server) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	sub := s.aggregator.Subscribe(status.Scope(req.Service), status.Concise)
	defer s.aggregator.Unsubscribe(sub)

	var lastServingStatus healthpb.HealthCheckResponse_ServingStatus = -1
	var failureTimer *time.Timer
	failureCh := make(chan struct{})

	for {
		select {
		case st, ok := <-sub:
			if !ok {
				return errShuttingDown
			}
			var sst healthpb.HealthCheckResponse_ServingStatus

			switch {
			case st == nil:
				sst = healthpb.HealthCheckResponse_SERVICE_UNKNOWN
			case s.componentHealthConfig.IncludeRecoverable &&
				s.componentHealthConfig.RecoveryDuration > 0 &&
				st.Status() == component.StatusRecoverableError:
				if failureTimer == nil {
					failureTimer = time.AfterFunc(
						s.componentHealthConfig.RecoveryDuration,
						func() { failureCh <- struct{}{} },
					)
				}
				sst = lastServingStatus
				if lastServingStatus == -1 {
					sst = healthpb.HealthCheckResponse_SERVING
				}
			default:
				if failureTimer != nil {
					if !failureTimer.Stop() {
						<-failureTimer.C
					}
					failureTimer = nil
				}
				sst = s.toServingStatus(st.Event)
			}

			if lastServingStatus == sst {
				continue
			}

			lastServingStatus = sst

			err := stream.Send(&healthpb.HealthCheckResponse{Status: sst})
			if err != nil {
				return errStreamSend
			}
		case <-failureCh:
			failureTimer.Stop()
			failureTimer = nil
			if lastServingStatus == healthpb.HealthCheckResponse_NOT_SERVING {
				continue
			}
			lastServingStatus = healthpb.HealthCheckResponse_NOT_SERVING
			err := stream.Send(
				&healthpb.HealthCheckResponse{
					Status: healthpb.HealthCheckResponse_NOT_SERVING,
				},
			)
			if err != nil {
				return errStreamSend
			}
		case <-stream.Context().Done():
			return errStreamEnded
		}
	}
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
