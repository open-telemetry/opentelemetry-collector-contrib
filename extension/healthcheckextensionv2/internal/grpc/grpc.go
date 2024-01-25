// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/grpc"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func (s *Server) Check(
	_ context.Context,
	req *healthpb.HealthCheckRequest,
) (*healthpb.HealthCheckResponse, error) {
	st, ok := s.aggregator.AggregateStatus(req.Service, false)
	if !ok {
		return nil, status.Error(codes.NotFound, "unknown service")
	}

	return &healthpb.HealthCheckResponse{
		Status: s.toServingStatus(st.StatusEvent),
	}, nil
}

func (s *Server) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	sub := s.aggregator.Subscribe(req.Service, false)
	defer s.aggregator.Unsubscribe(sub)

	var lastServingStatus healthpb.HealthCheckResponse_ServingStatus = -1

	failureTicker := time.NewTicker(s.recoveryDuration)
	failureTicker.Stop()

	for {
		select {
		case st, ok := <-sub:
			if !ok {
				return status.Error(codes.Canceled, "Server shutting down.")
			}
			var sst healthpb.HealthCheckResponse_ServingStatus

			switch {
			case st == nil:
				sst = healthpb.HealthCheckResponse_SERVICE_UNKNOWN
			case st.StatusEvent.Status() == component.StatusRecoverableError:
				failureTicker.Reset(s.recoveryDuration)
				sst = lastServingStatus
				if lastServingStatus == -1 {
					sst = healthpb.HealthCheckResponse_SERVING
				}
			default:
				failureTicker.Stop()
				sst = statusToServingStatusMap[st.StatusEvent.Status()]
			}

			if lastServingStatus == sst {
				continue
			}

			lastServingStatus = sst

			err := stream.Send(&healthpb.HealthCheckResponse{Status: sst})
			if err != nil {
				return status.Error(codes.Canceled, "Stream has ended.")
			}
		case <-failureTicker.C:
			failureTicker.Stop()
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
				return status.Error(codes.Canceled, "Stream has ended.")
			}
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "Stream has ended.")
		}
	}
}

var statusToServingStatusMap = map[component.Status]healthpb.HealthCheckResponse_ServingStatus{
	component.StatusNone:             healthpb.HealthCheckResponse_NOT_SERVING,
	component.StatusStarting:         healthpb.HealthCheckResponse_NOT_SERVING,
	component.StatusOK:               healthpb.HealthCheckResponse_SERVING,
	component.StatusRecoverableError: healthpb.HealthCheckResponse_SERVING,
	component.StatusPermanentError:   healthpb.HealthCheckResponse_NOT_SERVING,
	component.StatusFatalError:       healthpb.HealthCheckResponse_NOT_SERVING,
	component.StatusStopping:         healthpb.HealthCheckResponse_NOT_SERVING,
	component.StatusStopped:          healthpb.HealthCheckResponse_NOT_SERVING,
}

func (s *Server) toServingStatus(
	ev *component.StatusEvent,
) healthpb.HealthCheckResponse_ServingStatus {
	if ev.Status() == component.StatusRecoverableError &&
		time.Now().Compare(ev.Timestamp().Add(s.recoveryDuration)) == 1 {
		return healthpb.HealthCheckResponse_NOT_SERVING
	}
	return statusToServingStatusMap[ev.Status()]
}
