// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package httpdreceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpdreceiver/internal/metadata"
)

func TestHttpdIntegration(t *testing.T) {
	container := getContainer(t, containerRequest2_4)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()
	hostname, err := container.Host(context.Background())
	require.NoError(t, err)

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Endpoint = fmt.Sprintf("http://%s", net.JoinHostPort(hostname, "8080"))

	consumer := new(consumertest.MetricsSink)
	settings := componenttest.NewNopReceiverCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")

	md := consumer.AllMetrics()[0]
	require.Equal(t, 1, md.ResourceMetrics().Len())
	ilms := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilms.Len())
	metrics := ilms.At(0).Metrics()
	require.NoError(t, rcvr.Shutdown(context.Background()))

	validateResult(t, metrics)
}

var (
	containerRequest2_4 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join(".", "testdata"),
			Dockerfile: "Dockerfile.httpd",
		},
		ExposedPorts: []string{"8080:80"},
		WaitingFor:   waitStrategy{},
	}
)

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)
	return container
}

func validateResult(t *testing.T, metrics pdata.MetricSlice) {
	require.EqualValues(t, len(metadata.M.Names()), metrics.Len())

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)

		switch m.Name() {
		case metadata.M.HttpdUptime.Name():
			require.Equal(t, 1, m.Sum().DataPoints().Len())
			serverName, _ := m.Sum().DataPoints().At(0).Attributes().Get(metadata.L.ServerName)
			require.EqualValues(t, "localhost", serverName.AsString())
		case metadata.M.HttpdCurrentConnections.Name():
			require.Equal(t, 1, m.Gauge().DataPoints().Len())
			serverName, _ := m.Gauge().DataPoints().At(0).Attributes().Get(metadata.L.ServerName)
			require.EqualValues(t, "localhost", serverName.AsString())
		case metadata.M.HttpdWorkers.Name():
			require.Equal(t, 2, m.Gauge().DataPoints().Len())
			serverName, _ := m.Gauge().DataPoints().At(0).Attributes().Get(metadata.L.ServerName)
			require.EqualValues(t, "localhost", serverName.AsString())
			dps := m.Gauge().DataPoints()
			present := map[pdata.AttributeValue]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				serverName, _ := dp.Attributes().Get(metadata.L.ServerName)
				require.EqualValues(t, "localhost", serverName.AsString())
				state, _ := dp.Attributes().Get("state")
				switch state.AsString() {
				case metadata.LabelWorkersState.Busy:
					present[state] = true
				case metadata.LabelWorkersState.Idle:
					present[state] = true
				default:
					require.Nil(t, state, fmt.Sprintf("connections with state %s not expected", state.AsString()))
				}
			}
			require.Equal(t, 2, len(present))
		case metadata.M.HttpdRequests.Name():
			serverName, _ := m.Sum().DataPoints().At(0).Attributes().Get(metadata.L.ServerName)
			require.EqualValues(t, "localhost", serverName.AsString())
			require.Equal(t, 1, m.Sum().DataPoints().Len())
			require.True(t, m.Sum().IsMonotonic())
		case metadata.M.HttpdTraffic.Name():
			serverName, _ := m.Sum().DataPoints().At(0).Attributes().Get(metadata.L.ServerName)
			require.EqualValues(t, "localhost", serverName.AsString())
			require.Equal(t, 1, m.Sum().DataPoints().Len())
			require.True(t, m.Sum().IsMonotonic())
		case metadata.M.HttpdScoreboard.Name():
			dps := m.Gauge().DataPoints()
			require.Equal(t, 11, dps.Len())
			present := map[pdata.AttributeValue]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				serverName, _ := dp.Attributes().Get(metadata.L.ServerName)
				require.EqualValues(t, "localhost", serverName.AsString())
				state, _ := dp.Attributes().Get("state")
				switch state.AsString() {
				case metadata.LabelScoreboardState.Waiting:
					present[state] = true
				case metadata.LabelScoreboardState.Starting:
					present[state] = true
				case metadata.LabelScoreboardState.Reading:
					present[state] = true
				case metadata.LabelScoreboardState.Sending:
					present[state] = true
				case metadata.LabelScoreboardState.Keepalive:
					present[state] = true
				case metadata.LabelScoreboardState.Dnslookup:
					present[state] = true
				case metadata.LabelScoreboardState.Closing:
					present[state] = true
				case metadata.LabelScoreboardState.Logging:
					present[state] = true
				case metadata.LabelScoreboardState.Finishing:
					present[state] = true
				case metadata.LabelScoreboardState.IdleCleanup:
					present[state] = true
				case metadata.LabelScoreboardState.Open:
					present[state] = true
				default:
					require.Nil(t, state, fmt.Sprintf("connections with state %s not expected", state.AsString()))
				}
			}
			require.Equal(t, 11, len(present))
		default:
			require.Nil(t, m.Name(), fmt.Sprintf("metrics %s not expected", m.Name()))
		}

	}

}

type waitStrategy struct{}

func (ws waitStrategy) WaitUntilReady(ctx context.Context, st wait.StrategyTarget) error {
	if err := wait.ForListeningPort("80").
		WithStartupTimeout(2*time.Minute).
		WaitUntilReady(ctx, st); err != nil {
		return err
	}

	hostname, err := st.Host(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return fmt.Errorf("server startup problem")
		case <-time.After(100 * time.Millisecond):
			resp, err := http.Get(fmt.Sprintf("http://%s:8080/server-status?auto", hostname))
			if err != nil {
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				continue
			}

			if resp.Body.Close() != nil {
				continue
			}

			// The server needs a moment to generate some stats
			if strings.Contains(string(body), "ReqPerSec") {
				return nil
			}
		}
	}
}
