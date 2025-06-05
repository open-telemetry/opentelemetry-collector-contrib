// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/jaegertracing/jaeger-idl/proto-gen/api_v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

func TestRemote(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		remoteClientHeaderConfig      map[string]configopaque.String
		performedClientCallCount      int
		expectedOutboundGrpcCallCount int
		reloadInterval                time.Duration
	}{
		{
			name:                          "no configured header additions and no configured reload_interval",
			performedClientCallCount:      3,
			expectedOutboundGrpcCallCount: 3,
		},
		{
			name:                          "configured header additions",
			performedClientCallCount:      3,
			expectedOutboundGrpcCallCount: 3,
			remoteClientHeaderConfig: map[string]configopaque.String{
				"testheadername":    "testheadervalue",
				"anotherheadername": "anotherheadervalue",
			},
		},
		{
			name:                          "reload_interval set to nonzero value caching outbound same-service gRPC calls",
			reloadInterval:                time.Minute * 5,
			performedClientCallCount:      3,
			expectedOutboundGrpcCallCount: 1,
			remoteClientHeaderConfig: map[string]configopaque.String{
				"somecoolheader": "some-more-coverage-whynot",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// prepare the socket the mock server will listen at
			lis, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)

			// create the mock server
			server := grpc.NewServer()

			// register the service
			mockServer := &samplingServer{}
			api_v2.RegisterSamplingManagerServer(server, mockServer)

			go func() {
				err = server.Serve(lis)
				assert.NoError(t, err)
			}()
			defer func() { server.Stop() }()

			// create the config, pointing to the mock server
			cfg := testConfig()
			cfg.GRPCServerConfig.NetAddr.Endpoint = "127.0.0.1:0"
			cfg.Source.ReloadInterval = tc.reloadInterval
			cfg.Source.Remote = &configgrpc.ClientConfig{
				Endpoint: fmt.Sprintf("127.0.0.1:%d", lis.Addr().(*net.TCPAddr).Port),
				TLS: configtls.ClientConfig{
					Insecure: true, // test only
				},
				WaitForReady: true,
				Headers:      tc.remoteClientHeaderConfig,
			}

			// create the extension
			e := newExtension(cfg, componenttest.NewNopTelemetrySettings())
			require.NotNil(t, e)

			// start the server
			assert.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

			// make test case defined number of calls
			for i := 0; i < tc.performedClientCallCount; i++ {
				resp, err := http.Get("http://127.0.0.1:5778/sampling?service=foo")
				assert.NoError(t, err)
				assert.Equal(t, 200, resp.StatusCode)
				assert.NoError(t, resp.Body.Close())
			}

			// shut down the server
			assert.NoError(t, e.Shutdown(context.Background()))

			// verify observed calls
			assert.Len(t, mockServer.observedCalls, tc.expectedOutboundGrpcCallCount)
			for _, singleCall := range mockServer.observedCalls {
				assert.Equal(t, &api_v2.SamplingStrategyParameters{
					ServiceName: "foo",
				}, singleCall.params)
				md, ok := metadata.FromIncomingContext(singleCall.ctx)
				assert.True(t, ok)
				for expectedHeaderName, expectedHeaderValue := range tc.remoteClientHeaderConfig {
					assert.Equal(t, []string{string(expectedHeaderValue)}, md.Get(expectedHeaderName))
				}
			}
		})
	}
}

type samplingServer struct {
	api_v2.UnimplementedSamplingManagerServer
	observedCalls []observedCall
}

type observedCall struct {
	ctx    context.Context
	params *api_v2.SamplingStrategyParameters
}

func (s *samplingServer) GetSamplingStrategy(ctx context.Context, params *api_v2.SamplingStrategyParameters) (*api_v2.SamplingStrategyResponse, error) {
	s.observedCalls = append(s.observedCalls, observedCall{
		ctx:    ctx,
		params: params,
	})
	return &api_v2.SamplingStrategyResponse{
		StrategyType: api_v2.SamplingStrategyType_PROBABILISTIC,
	}, nil
}

func testConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPServerConfig.Endpoint = "127.0.0.1:5778"
	cfg.GRPCServerConfig.NetAddr.Endpoint = "127.0.0.1:14250"
	return cfg
}
