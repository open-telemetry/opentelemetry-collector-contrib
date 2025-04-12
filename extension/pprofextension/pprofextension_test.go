// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofextension

import (
	"context"
	"net"
	"net/http"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestPerformanceProfilerExtensionUsage(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddrConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		BlockProfileFraction: 3,
		MutexProfileFraction: 5,
	}
	tt := componenttest.NewTelemetry()

	pprofExt := newServer(config, tt.NewTelemetrySettings())
	require.NotNil(t, pprofExt)

	require.NoError(t, pprofExt.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, pprofExt.Shutdown(context.Background())) })

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	_, pprofPort, err := net.SplitHostPort(config.TCPAddr.Endpoint)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Get("http://localhost:" + pprofPort + "/debug/pprof")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPerformanceProfilerExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	config := Config{
		TCPAddr: confignet.TCPAddrConfig{
			Endpoint: endpoint,
		},
	}
	tt := componenttest.NewTelemetry()
	pprofExt := newServer(config, tt.NewTelemetrySettings())
	require.NotNil(t, pprofExt)

	require.Error(t, pprofExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestPerformanceProfilerMultipleStarts(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddrConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
	}

	tt := componenttest.NewTelemetry()
	pprofExt := newServer(config, tt.NewTelemetrySettings())
	require.NotNil(t, pprofExt)

	require.NoError(t, pprofExt.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, pprofExt.Shutdown(context.Background())) })

	// The instance is already active it will fail.
	require.Error(t, pprofExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestPerformanceProfilerMultipleShutdowns(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddrConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
	}

	tt := componenttest.NewTelemetry()
	pprofExt := newServer(config, tt.NewTelemetrySettings())
	require.NotNil(t, pprofExt)

	require.NoError(t, pprofExt.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, pprofExt.Shutdown(context.Background()))
	require.NoError(t, pprofExt.Shutdown(context.Background()))
}

func TestPerformanceProfilerShutdownWithoutStart(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddrConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
	}
	tt := componenttest.NewTelemetry()
	pprofExt := newServer(config, tt.NewTelemetrySettings())
	require.NotNil(t, pprofExt)

	require.NoError(t, pprofExt.Shutdown(context.Background()))
}

func TestPerformanceProfilerLifecycleWithFile(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "pprof*.yaml")
	require.NoError(t, err)
	defer func() {
		os.Remove(tmpFile.Name())
	}()
	require.NoError(t, tmpFile.Close())

	config := Config{
		TCPAddr: confignet.TCPAddrConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		SaveToFile: tmpFile.Name(),
	}
	tt := componenttest.NewTelemetry()
	pprofExt := newServer(config, tt.NewTelemetrySettings())
	require.NotNil(t, pprofExt)

	require.NoError(t, pprofExt.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, pprofExt.Shutdown(context.Background()))
}
