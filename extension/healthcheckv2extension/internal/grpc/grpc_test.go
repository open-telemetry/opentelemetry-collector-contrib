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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/testhelpers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestCheck(t *testing.T) {
	addr := testutil.GetAvailableLocalAddress(t)
	config := &Config{
		ServerConfig: configgrpc.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint:  addr,
				Transport: "tcp",
			},
		},
	}
	var server *Server
	traces := testhelpers.NewPipelineMetadata("traces")
	metrics := testhelpers.NewPipelineMetadata("metrics")

	type teststep struct {
		step           func()
		eventually     bool
		service        string
		expectedStatus healthpb.HealthCheckResponse_ServingStatus
		expectedErr    error
	}

	tests := []struct {
		name                    string
		config                  *Config
		componentHealthSettings *common.ComponentHealthConfig
		teststeps               []teststep
	}{
		{
			name:   "exclude recoverable and permanent errors",
			config: config,
			teststeps: []teststep{
				{
					service:        "",
					expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
				},
				{
					service:     traces.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
				},
				{
					service:     metrics.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
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
						// errors will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							component.NewRecoverableErrorEvent(assert.AnError),
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
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							component.NewStatusEvent(component.StatusOK),
						)
					},
					service:        "",
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
			},
		},
		{
			name:   "include recoverable and exclude permanent errors",
			config: config,
			componentHealthSettings: &common.ComponentHealthConfig{
				IncludePermanent:   false,
				IncludeRecoverable: true,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
				{
					service:        "",
					expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
				},
				{
					service:     traces.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
				},
				{
					service:     metrics.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
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
						// permament error will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							component.NewPermanentErrorEvent(assert.AnError),
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
			},
		},
		{
			name:   "include permanent and exclude recoverable errors",
			config: config,
			componentHealthSettings: &common.ComponentHealthConfig{
				IncludePermanent: true,
			},
			teststeps: []teststep{
				{
					service:        "",
					expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
				},
				{
					service:     traces.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
				},
				{
					service:     metrics.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
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
						// recoverable will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							component.NewRecoverableErrorEvent(assert.AnError),
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
						// permament error included
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							component.NewPermanentErrorEvent(assert.AnError),
						)
					},
					service:        "",
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
			},
		},
		{
			name:   "include permanent and recoverable errors",
			config: config,
			componentHealthSettings: &common.ComponentHealthConfig{
				IncludePermanent:   true,
				IncludeRecoverable: true,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
				{
					service:        "",
					expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
				},
				{
					service:     traces.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
				},
				{
					service:     metrics.PipelineID.String(),
					expectedErr: grpcstatus.Error(codes.NotFound, "Service not found."),
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
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server = NewServer(
				config,
				tc.componentHealthSettings,
				componenttest.NewNopTelemetrySettings(),
				status.NewAggregator(testhelpers.ErrPriority(tc.componentHealthSettings)),
			)
			require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
			t.Cleanup(func() { require.NoError(t, server.Shutdown(context.Background())) })

			cc, err := grpc.NewClient(
				addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			defer func() {
				assert.NoError(t, cc.Close())
			}()

			client := healthpb.NewHealthClient(cc)

			for _, ts := range tc.teststeps {
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
		})
	}

}
