// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/testhelpers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestCheck(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	settings := &Settings{
		GRPCServerSettings: configgrpc.GRPCServerSettings{
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: "tcp",
			},
		},
	}
	server := NewServer(
		settings,
		componenttest.NewNopTelemetrySettings(),
		10*time.Millisecond,
		status.NewAggregator(),
	)
	traces := testhelpers.NewPipelineMetadata("traces")
	metrics := testhelpers.NewPipelineMetadata("metrics")

	require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, server.Shutdown(context.Background())) })

	cc, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cc.Close())
	}()

	client := healthpb.NewHealthClient(cc)

	// ts is a sequence of test steps
	for _, ts := range []struct {
		step           func()
		eventually     bool
		service        string
		expectedStatus healthpb.HealthCheckResponse_ServingStatus
		expectedErr    error
	}{
		{
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:     traces.PipelineID.String(),
			expectedErr: grpcstatus.Error(codes.NotFound, "unknown service"),
		},
		{
			service:     metrics.PipelineID.String(),
			expectedErr: grpcstatus.Error(codes.NotFound, "unknown service"),
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusStarting,
				)
				testhelpers.SeedAggregator(
					server.aggregator,
					metrics.InstanceIDs(),
					component.StatusStarting,
				)
			},
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusOK,
				)
				testhelpers.SeedAggregator(
					server.aggregator,
					metrics.InstanceIDs(),
					component.StatusOK,
				)
			},
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			step: func() {
				// metrics and overall status will be NOT_SERVING
				server.aggregator.RecordStatus(
					metrics.ExporterID,
					component.NewRecoverableErrorEvent(assert.AnError),
				)
			},
			service:        "",
			eventually:     true,
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			step: func() {
				// metrics and overall status will recover and resume SERVING
				server.aggregator.RecordStatus(
					metrics.ExporterID,
					component.NewStatusEvent(component.StatusOK),
				)
			},
			service:        "",
			eventually:     true,
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusStopping,
				)
				testhelpers.SeedAggregator(
					server.aggregator,
					metrics.InstanceIDs(),
					component.StatusStopping,
				)
			},
			service:        "",
			eventually:     true,
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusStopped,
				)
				testhelpers.SeedAggregator(
					server.aggregator,
					metrics.InstanceIDs(),
					component.StatusStopped,
				)
			},
			service:        "",
			eventually:     true,
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
	} {
		if ts.step != nil {
			ts.step()
		}

		if ts.eventually {
			assert.Eventually(t, func() bool {
				resp, err := client.Check(
					context.Background(),
					&healthpb.HealthCheckRequest{Service: ts.service},
				)
				require.NoError(t, err)
				return ts.expectedStatus == resp.Status
			}, time.Second, 10*time.Millisecond)
			continue
		}

		resp, err := client.Check(
			context.Background(),
			&healthpb.HealthCheckRequest{Service: ts.service},
		)
		require.Equal(t, ts.expectedErr, err)
		if ts.expectedErr != nil {
			continue
		}
		assert.Equal(t, ts.expectedStatus, resp.Status)
	}
}

func TestWatch(t *testing.T) {
	var err error
	addr := testutil.GetAvailableLocalAddress(t)
	settings := &Settings{
		GRPCServerSettings: configgrpc.GRPCServerSettings{
			NetAddr: confignet.NetAddr{
				Endpoint:  addr,
				Transport: "tcp",
			},
		},
	}
	server := NewServer(
		settings,
		componenttest.NewNopTelemetrySettings(),
		10*time.Millisecond,
		status.NewAggregator(),
	)
	traces := testhelpers.NewPipelineMetadata("traces")
	metrics := testhelpers.NewPipelineMetadata("metrics")

	require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, server.Shutdown(context.Background())) })

	cc, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, cc.Close())
	}()

	client := healthpb.NewHealthClient(cc)
	watchers := make(map[string]healthpb.Health_WatchClient)

	// ts is a sequence of test steps
	for _, ts := range []struct {
		step           func()
		service        string
		expectedStatus healthpb.HealthCheckResponse_ServingStatus
	}{
		{
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVICE_UNKNOWN,
		},
		{
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVICE_UNKNOWN,
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusStarting,
				)
			},
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					metrics.InstanceIDs(),
					component.StatusStarting,
				)
			},
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusOK,
				)
			},
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					metrics.InstanceIDs(),
					component.StatusOK,
				)
			},
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			step: func() {
				// metrics and overall status will be NOT_SERVING
				server.aggregator.RecordStatus(
					metrics.ExporterID,
					component.NewRecoverableErrorEvent(assert.AnError),
				)
			},
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			step: func() {
				// metrics and overall status will recover and resume SERVING
				server.aggregator.RecordStatus(
					metrics.ExporterID,
					component.NewStatusEvent(component.StatusOK),
				)
			},
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			step: func() {
				// This will be the last status change for traces (stopping changes to NOT_SERVING)
				// Stopped results in the same serving status, and repeat statuses are not streamed.
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusStopping,
				)
			},
			service:        traces.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			service:        "",
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			step: func() {
				// This will be the last status change for metrics (stopping changes to NOT_SERVING)
				// Stopped results in the same serving status, and repeat statuses are not streamed.
				testhelpers.SeedAggregator(
					server.aggregator,
					metrics.InstanceIDs(),
					component.StatusStopping,
				)
			},
			service:        metrics.PipelineID.String(),
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
	} {

		if ts.step != nil {
			ts.step()
		}

		watcher, ok := watchers[ts.service]
		if !ok {
			watcher, err = client.Watch(
				context.Background(),
				&healthpb.HealthCheckRequest{Service: ts.service},
			)
			require.NoError(t, err)
			watchers[ts.service] = watcher
		}

		var resp *healthpb.HealthCheckResponse
		// Note Recv blocks until there is a new item in the stream
		resp, err = watcher.Recv()
		require.NoError(t, err)
		assert.Equal(t, ts.expectedStatus, resp.Status)
	}

	// closing the aggregator will gracefully terminate streams of status events
	server.aggregator.Close()

	// Ensure watchers receive the cancelation when streams are closed by the server
	for _, watcher := range watchers {
		_, err = watcher.Recv()
		assert.Equal(t, grpcstatus.Error(codes.Canceled, "Server shutting down."), err)
	}
}
