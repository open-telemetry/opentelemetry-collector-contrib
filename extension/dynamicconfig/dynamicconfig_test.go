// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynamicconfig

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/service/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/testutil"

	pb "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/proto/experimental/metrics/configservice"
)

func TestDynamicConfigExtensionUsage(t *testing.T) {
	config := Config{
		Endpoint:        testutil.GetAvailableLocalAddress(t),
		LocalConfigFile: "testdata/schedules.yaml",
	}

	dynamicconfigExt, err := newServer(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, dynamicconfigExt)

	require.NoError(t, dynamicconfigExt.Start(context.Background(), componenttest.NewNopHost()))
	defer dynamicconfigExt.Shutdown(context.Background())

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	conn, err := grpc.Dial(
		config.Endpoint,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Errorf("fail to open connection: %v", err)
	}
	defer func() {
		if e := conn.Close(); e != nil {
			t.Errorf("fail to close connection: %v", err)
		}
	}()

	client := pb.NewMetricConfigClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = client.GetMetricConfig(ctx, &pb.MetricConfigRequest{})
	require.NoError(t, err)
}

func TestDynamicConfigExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	config := Config{
		Endpoint:        endpoint,
		LocalConfigFile: "testdata/schedules.yaml",
	}
	dynamicconfigExt, err := newServer(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, dynamicconfigExt)

	require.Error(t, dynamicconfigExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestDynamicConfigMultipleStarts(t *testing.T) {
	config := Config{
		Endpoint:        testutil.GetAvailableLocalAddress(t),
		LocalConfigFile: "testdata/schedules.yaml",
	}

	dynamicconfigExt, err := newServer(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, dynamicconfigExt)

	require.NoError(t, dynamicconfigExt.Start(context.Background(), componenttest.NewNopHost()))
	defer dynamicconfigExt.Shutdown(context.Background())

	// Try to start it again, it will fail since it is on the same endpoint.
	require.Error(t, dynamicconfigExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestDynamicConfigMultipleShutdowns(t *testing.T) {
	config := Config{
		Endpoint:        testutil.GetAvailableLocalAddress(t),
		LocalConfigFile: "testdata/schedules.yaml",
	}

	dynamicconfigExt, err := newServer(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, dynamicconfigExt)

	require.NoError(t, dynamicconfigExt.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, dynamicconfigExt.Shutdown(context.Background()))
	require.NoError(t, dynamicconfigExt.Shutdown(context.Background()))
}

func TestDynamicConfigShutdownWithoutStart(t *testing.T) {
	config := Config{
		Endpoint:        testutil.GetAvailableLocalAddress(t),
		LocalConfigFile: "testdata/schedules.yaml",
	}

	dynamicconfigExt, err := newServer(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, dynamicconfigExt)

	require.NoError(t, dynamicconfigExt.Shutdown(context.Background()))
}

func TestDynamicConfigRemoteConnection(t *testing.T) {
	quit := make(chan struct{})
	done := make(chan struct{})

	servAddr := mock.StartServer(t, quit, done)
	config := Config{
		Endpoint:            testutil.GetAvailableLocalAddress(t),
		RemoteConfigAddress: servAddr,
	}

	dynamicconfigExt, err := newServer(config, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, dynamicconfigExt)

	require.NoError(t, dynamicconfigExt.Start(context.Background(), componenttest.NewNopHost()))
	defer dynamicconfigExt.Shutdown(context.Background())

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	conn, err := grpc.Dial(
		config.Endpoint,
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Errorf("fail to open connection: %v", err)
	}
	defer func() {
		if e := conn.Close(); e != nil {
			t.Errorf("fail to close connection: %v", err)
		}
	}()

	client := pb.NewMetricConfigClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = client.GetMetricConfig(ctx, &pb.MetricConfigRequest{})
	require.NoError(t, err)

	quit <- struct{}{}
}
