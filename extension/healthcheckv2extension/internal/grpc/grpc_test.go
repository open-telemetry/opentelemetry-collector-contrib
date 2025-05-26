// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pipeline"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	internalhelpers "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/testhelpers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status/testhelpers"
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
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)
	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)

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
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
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
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
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
							componentstatus.NewEvent(componentstatus.StatusOK),
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
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
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
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
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
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
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
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
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
							componentstatus.NewEvent(componentstatus.StatusOK),
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
						// permanent error will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
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
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
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
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
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
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
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
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
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
						// permanent error included
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
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
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
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
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
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
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
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
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
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
							componentstatus.NewEvent(componentstatus.StatusOK),
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
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
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
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
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
				status.NewAggregator(internalhelpers.ErrPriority(tc.componentHealthSettings)),
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

func TestWatch(t *testing.T) {
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
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)
	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)

	// statusUnchanged is a sentinel value to signal that a step does not result
	// in a status change. This is important, because checking for a status
	// change is blocking.
	var statusUnchanged healthpb.HealthCheckResponse_ServingStatus = -1

	type teststep struct {
		step           func()
		service        string
		expectedStatus healthpb.HealthCheckResponse_ServingStatus
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
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
							componentstatus.StatusOK,
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
						// errors will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: statusUnchanged,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: statusUnchanged,
				},
				{
					step: func() {
						// This will be the last status change for traces (stopping changes to NOT_SERVING)
						// Stopped results in the same serving status, and repeat statuses are not streamed.
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
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
							componentstatus.StatusStopping,
						)
					},
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
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
							componentstatus.StatusOK,
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
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
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
							componentstatus.NewEvent(componentstatus.StatusOK),
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
						// permanent error will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: statusUnchanged,
				},
			},
		},
		{
			name:   "exclude permanent errors",
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
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
							componentstatus.StatusOK,
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
						// permanent error will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: statusUnchanged,
				},
			},
		},
		{
			name:   "include recoverable 0s recovery duration",
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
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
							componentstatus.StatusOK,
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
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
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
							componentstatus.NewEvent(componentstatus.StatusOK),
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
							componentstatus.StatusStopping,
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
							componentstatus.StatusStopping,
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
				},
			},
		},
		{
			name:   "include permanent and exclude recoverable errors",
			config: config,
			componentHealthSettings: &common.ComponentHealthConfig{
				IncludePermanent:   true,
				IncludeRecoverable: false,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
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
							componentstatus.StatusOK,
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
						// recoverable will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: statusUnchanged,
				},
				{
					step: func() {
						// metrics and overall status will recover and resume SERVING
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
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
						// This will be the last status change for traces (stopping changes to NOT_SERVING)
						// Stopped results in the same serving status, and repeat statuses are not streamed.
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					service:        traces.PipelineID.String(),
					expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
				},
			},
		},
		{
			name:   "exclude recoverable errors",
			config: config,
			componentHealthSettings: &common.ComponentHealthConfig{
				IncludePermanent:   true,
				IncludeRecoverable: false,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
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
							componentstatus.StatusOK,
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
						// recoverable will be ignored
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: statusUnchanged,
				},
			},
		},
		{
			name:   "include recoverable and permanent errors",
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusStarting,
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
							componentstatus.StatusOK,
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
							componentstatus.StatusOK,
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
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
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
							componentstatus.NewEvent(componentstatus.StatusOK),
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
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					service:        metrics.PipelineID.String(),
					expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
				},
				{
					service:        "",
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
				status.NewAggregator(internalhelpers.ErrPriority(tc.componentHealthSettings)),
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
			watchers := make(map[string]healthpb.Health_WatchClient)

			for _, ts := range tc.teststeps {
				if ts.step != nil {
					ts.step()
				}

				if statusUnchanged == ts.expectedStatus {
					continue
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

			wg := sync.WaitGroup{}
			wg.Add(len(watchers))

			for svc, watcher := range watchers {
				go func() {
					resp, err := watcher.Recv()
					// Ensure there are not any unread messages
					assert.Nil(t, resp, "%s: had unread messages", svc)
					// Ensure watchers receive the cancelation when streams are closed by the server
					assert.Equal(t, grpcstatus.Error(codes.Canceled, "Server shutting down."), err)
					wg.Done()
				}()
			}

			// closing the aggregator will gracefully terminate streams of status events
			server.aggregator.Close()
			wg.Wait()
		})
	}
}
