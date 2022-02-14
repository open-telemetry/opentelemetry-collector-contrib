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

package healthcheckextension

import (
	"context"
	"net"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func ensureServerRunning(url string) func() bool {
	return func() bool {
		_, err := net.DialTimeout("tcp", url, 30*time.Second)
		return err == nil
	}
}

func TestHealthCheckExtensionUsageWithoutCheckCollectorPipeline(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}

	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	client := &http.Client{}
	url := "http://" + config.TCPAddr.Endpoint
	resp0, err := client.Get(url)
	require.NoError(t, err)
	defer resp0.Body.Close()

	require.Equal(t, http.StatusServiceUnavailable, resp0.StatusCode)

	require.NoError(t, hcExt.Ready())
	resp1, err := client.Get(url)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	require.NoError(t, hcExt.NotReady())
	resp2, err := client.Get(url)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp2.StatusCode)
}

func TestHealthCheckExtensionUsageWithCustomizedPathWithoutCheckCollectorPipeline(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/health",
	}

	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })
	require.Eventuallyf(t, ensureServerRunning(config.TCPAddr.Endpoint), 30*time.Second, 1*time.Second, "Failed to start the testing server.")

	client := &http.Client{}
	url := "http://" + config.TCPAddr.Endpoint + config.Path
	resp0, err := client.Get(url)
	require.NoError(t, err)
	require.NoError(t, resp0.Body.Close(), "Must be able to close the response")

	require.Equal(t, http.StatusServiceUnavailable, resp0.StatusCode)

	require.NoError(t, hcExt.Ready())
	resp1, err := client.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp1.StatusCode)
	require.NoError(t, resp1.Body.Close(), "Must be able to close the response")

	require.NoError(t, hcExt.NotReady())
	resp2, err := client.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp2.StatusCode)
	require.NoError(t, resp2.Body.Close(), "Must be able to close the response")
}

func TestHealthCheckExtensionUsageWithCheckCollectorPipeline(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: checkCollectorPipelineSettings{
			Enabled:                  true,
			Interval:                 "5m",
			ExporterFailureThreshold: 1,
		},
		Path: "/",
	}

	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })

	// Give a chance for the server goroutine to run.
	runtime.Gosched()

	newView := view.View{Name: exporterFailureView}

	currentTime := time.Now()
	vd1 := &view.Data{
		View:  &newView,
		Start: currentTime.Add(-2 * time.Minute),
		End:   currentTime,
		Rows:  nil,
	}
	vd2 := &view.Data{
		View:  &newView,
		Start: currentTime.Add(-1 * time.Minute),
		End:   currentTime,
		Rows:  nil,
	}

	client := &http.Client{}
	url := "http://" + config.TCPAddr.Endpoint
	resp0, err := client.Get(url)
	require.NoError(t, err)
	defer resp0.Body.Close()

	hcExt.exporter.exporterFailureQueue = append(hcExt.exporter.exporterFailureQueue, vd1)
	require.NoError(t, hcExt.Ready())
	resp1, err := client.Get(url)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	require.NoError(t, hcExt.NotReady())
	resp2, err := client.Get(url)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusInternalServerError, resp2.StatusCode)

	hcExt.exporter.exporterFailureQueue = append(hcExt.exporter.exporterFailureQueue, vd2)
	require.NoError(t, hcExt.Ready())
	resp3, err := client.Get(url)
	require.NoError(t, err)
	defer resp3.Body.Close()
	require.Equal(t, http.StatusInternalServerError, resp3.StatusCode)
}

func TestHealthCheckExtensionUsageWithCustomPathWithCheckCollectorPipeline(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: checkCollectorPipelineSettings{
			Enabled:                  true,
			Interval:                 "5m",
			ExporterFailureThreshold: 1,
		},
		Path: "/health",
	}

	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })

	// Give a chance for the server goroutine to run.
	runtime.Gosched()
	require.Eventuallyf(t, ensureServerRunning(config.TCPAddr.Endpoint), 30*time.Second, 1*time.Second, "Failed to start the testing server.")

	newView := view.View{Name: exporterFailureView}

	currentTime := time.Now()
	vd1 := &view.Data{
		View:  &newView,
		Start: currentTime.Add(-2 * time.Minute),
		End:   currentTime,
		Rows:  nil,
	}
	vd2 := &view.Data{
		View:  &newView,
		Start: currentTime.Add(-1 * time.Minute),
		End:   currentTime,
		Rows:  nil,
	}

	client := &http.Client{}
	url := "http://" + config.TCPAddr.Endpoint + config.Path
	resp0, err := client.Get(url)
	require.NoError(t, err)
	require.NoError(t, resp0.Body.Close(), "Must be able to close the response")

	hcExt.exporter.exporterFailureQueue = append(hcExt.exporter.exporterFailureQueue, vd1)
	require.NoError(t, hcExt.Ready())
	resp1, err := client.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp1.StatusCode)
	require.NoError(t, resp1.Body.Close(), "Must be able to close the response")

	require.NoError(t, hcExt.NotReady())
	resp2, err := client.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp2.StatusCode)
	require.NoError(t, resp2.Body.Close(), "Must be able to close the response")

	hcExt.exporter.exporterFailureQueue = append(hcExt.exporter.exporterFailureQueue, vd2)
	require.NoError(t, hcExt.Ready())
	resp3, err := client.Get(url)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp3.StatusCode)
	require.NoError(t, resp3.Body.Close(), "Must be able to close the response")
}

func TestHealthCheckExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)

	// This needs to be ":port" because health checks also tries to connect to ":port".
	// To avoid the pop-up "accept incoming network connections" health check should be changed
	// to accept an address.
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: endpoint,
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
	}
	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	mh := newAssertNoErrorHost(t)
	require.Error(t, hcExt.Start(context.Background(), mh))
}

func TestHealthCheckMultipleStarts(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}

	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	mh := newAssertNoErrorHost(t)
	require.NoError(t, hcExt.Start(context.Background(), mh))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })

	require.Error(t, hcExt.Start(context.Background(), mh))
}

func TestHealthCheckMultipleShutdowns(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}

	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, hcExt.Shutdown(context.Background()))
	require.NoError(t, hcExt.Shutdown(context.Background()))
}

func TestHealthCheckShutdownWithoutStart(t *testing.T) {
	config := Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
	}

	hcExt := newServer(config, zap.NewNop())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Shutdown(context.Background()))
}

// assertNoErrorHost implements a component.Host that asserts that there were no errors.
type assertNoErrorHost struct {
	component.Host
	*testing.T
}

// newAssertNoErrorHost returns a new instance of assertNoErrorHost.
func newAssertNoErrorHost(t *testing.T) component.Host {
	return &assertNoErrorHost{
		Host: componenttest.NewNopHost(),
		T:    t,
	}
}

func (aneh *assertNoErrorHost) ReportFatalError(err error) {
	assert.NoError(aneh, err)
}
