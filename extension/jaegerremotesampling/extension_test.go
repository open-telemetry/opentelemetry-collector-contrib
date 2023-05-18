// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"

	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"google.golang.org/grpc"
)

func TestNewExtension(t *testing.T) {
	// test
	cfg := testConfig()
	cfg.Source.File = filepath.Join("testdata", "strategy.json")
	e := newExtension(cfg, componenttest.NewNopTelemetrySettings())

	// verify
	assert.NotNil(t, e)
}

func TestStartAndShutdownLocalFile(t *testing.T) {
	// prepare
	cfg := testConfig()
	cfg.Source.File = filepath.Join("testdata", "strategy.json")

	e := newExtension(cfg, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, e)
	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	// test and verify
	assert.NoError(t, e.Shutdown(context.Background()))
}

func TestStartAndShutdownRemote(t *testing.T) {
	// prepare the socket the mock server will listen at
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// create the mock server
	server := grpc.NewServer()

	// register the service
	api_v2.RegisterSamplingManagerServer(server, &samplingServer{})

	go func() {
		err = server.Serve(lis)
		require.NoError(t, err)
	}()

	// create the config, pointing to the mock server
	cfg := testConfig()
	cfg.GRPCServerSettings.NetAddr.Endpoint = "127.0.0.1:0"
	cfg.Source.Remote = &configgrpc.GRPCClientSettings{
		Endpoint:     fmt.Sprintf("127.0.0.1:%d", lis.Addr().(*net.TCPAddr).Port),
		WaitForReady: true,
	}

	// create the extension
	e := newExtension(cfg, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, e)

	// test
	assert.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, e.Shutdown(context.Background()))
}

type samplingServer struct {
	api_v2.UnimplementedSamplingManagerServer
}

func (s samplingServer) GetSamplingStrategy(ctx context.Context, param *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {
	return &api_v2.SamplingStrategyResponse{
		StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC,
	}, nil
}

func testConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPServerSettings.Endpoint = "127.0.0.1:5778"
	cfg.GRPCServerSettings.NetAddr.Endpoint = "127.0.0.1:14250"
	return cfg
}
