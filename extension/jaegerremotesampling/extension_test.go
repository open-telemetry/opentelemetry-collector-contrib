// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	cfg := createDefaultConfig().(*Config)
	cfg.Source.File = filepath.Join("testdata", "strategy.json")
	e := newExtension(cfg, componenttest.NewNopTelemetrySettings())

	// verify
	assert.NotNil(t, e)
}

func TestStartAndShutdownLocalFile(t *testing.T) {
	// prepare
	cfg := createDefaultConfig().(*Config)
	cfg.Source.File = filepath.Join("testdata", "strategy.json")

	e := newExtension(cfg, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, e)
	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	// test and verify
	assert.NoError(t, e.Shutdown(context.Background()))
}

func TestStartAndShutdownRemote(t *testing.T) {
	// prepare the socket the mock server will listen at
	lis, err := net.Listen("tcp", "localhost:0")
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
	cfg := createDefaultConfig().(*Config)
	cfg.Source.Remote = &configgrpc.GRPCClientSettings{
		Endpoint:     fmt.Sprintf("localhost:%d", lis.Addr().(*net.TCPAddr).Port),
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
