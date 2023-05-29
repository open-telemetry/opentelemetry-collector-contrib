// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
)

func TestMissingClientConfigManagerGRPC(t *testing.T) {
	s, err := NewGRPC(componenttest.NewNopTelemetrySettings(), configgrpc.GRPCServerSettings{}, nil)
	assert.Equal(t, errMissingStrategyStore, err)
	assert.Nil(t, s)
}

func TestStartAndStopGRPC(t *testing.T) {
	// prepare
	srvSettings := configgrpc.GRPCServerSettings{
		NetAddr: confignet.NetAddr{
			Endpoint:  "127.0.0.1:0",
			Transport: "tcp",
		},
	}
	s, err := NewGRPC(componenttest.NewNopTelemetrySettings(), srvSettings, &mockCfgMgr{})
	require.NoError(t, err)
	require.NotNil(t, s)

	// test
	assert.NoError(t, s.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, s.Shutdown(context.Background()))
}

func TestSamplingGRPCServer_Shutdown(t *testing.T) {
	tt := []struct {
		name       string
		grpcServer grpcServer
		timeout    time.Duration
		expect     error
	}{
		{
			name:       "graceful stop is successful without delay",
			grpcServer: &grpcServerMock{},
			timeout:    time.Minute,
		},
		{
			name: "graceful stop is successful with delay",
			grpcServer: &grpcServerMock{
				timeToGracefulStop: 5 * time.Second,
			},
			timeout: time.Minute,
		},
		{
			name: "context timed out",
			grpcServer: &grpcServerMock{
				timeToGracefulStop: time.Minute,
			},
			timeout: 5 * time.Second,
		},
		{
			name:    "grpc server not started",
			timeout: time.Minute,
			expect:  errGRPCServerNotRunning,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			srv := &SamplingGRPCServer{grpcServer: tc.grpcServer}
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()
			assert.Equal(t, tc.expect, srv.Shutdown(ctx))
		})
	}
}

type grpcServerMock struct {
	timeToGracefulStop time.Duration
}

func (g *grpcServerMock) Serve(lis net.Listener) error { return nil }
func (g *grpcServerMock) Stop()                        {}
func (g *grpcServerMock) GracefulStop()                { time.Sleep(g.timeToGracefulStop) }
