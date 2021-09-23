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

//go:build integration
// +build integration

package nginxreceiver

import (
	"context"
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper"
)

type NginxIntegrationSuite struct {
	suite.Suite
}

func TestNginxIntegration(t *testing.T) {
	suite.Run(t, new(NginxIntegrationSuite))
}

func nginxContainer(t *testing.T) testcontainers.Container {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join(".", "testdata"),
			Dockerfile: "Dockerfile.nginx",
		},
		ExposedPorts: []string{"8080:80"},
		WaitingFor:   wait.ForListeningPort("80"),
	}

	require.NoError(t, req.Validate())

	nginx, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	time.Sleep(time.Second * 6)
	return nginx
}

func (suite *NginxIntegrationSuite) TestNginxScraperHappyPath() {
	t := suite.T()
	nginx := nginxContainer(t)
	defer nginx.Terminate(context.Background())
	hostname, err := nginx.Host(context.Background())
	require.NoError(t, err)

	cfg := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 100 * time.Millisecond,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: fmt.Sprintf("http://%s:8080/status", hostname),
		},
	}

	sc := newNginxScraper(zap.NewNop(), cfg)
	err = sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	rms, err := sc.scrape(context.Background())
	require.Nil(t, err)

	require.Equal(t, 1, rms.Len())
	rm := rms.At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilms.Len())

	ilm := ilms.At(0)
	ms := ilm.Metrics()

	require.Equal(t, 4, ms.Len())

	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)

		switch m.Name() {
		case metadata.M.NginxRequests.Name():
			require.Equal(t, 1, m.Sum().DataPoints().Len())
			require.True(t, m.Sum().IsMonotonic())
		case metadata.M.NginxConnectionsAccepted.Name():
			require.Equal(t, 1, m.Sum().DataPoints().Len())
			require.True(t, m.Sum().IsMonotonic())
		case metadata.M.NginxConnectionsHandled.Name():
			require.Equal(t, 1, m.Sum().DataPoints().Len())
			require.True(t, m.Sum().IsMonotonic())
		case metadata.M.NginxConnectionsCurrent.Name():
			dps := m.Gauge().DataPoints()
			require.Equal(t, 4, dps.Len())
			present := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				state, ok := dp.Attributes().Get("state")
				if !ok {
					continue
				}
				switch state.StringVal() {
				case metadata.LabelState.Active:
					present[state.StringVal()] = true
				case metadata.LabelState.Reading:
					present[state.StringVal()] = true
				case metadata.LabelState.Writing:
					present[state.StringVal()] = true
				case metadata.LabelState.Waiting:
					present[state.StringVal()] = true
				default:
					t.Error(fmt.Sprintf("connections with state %s not expected", state.StringVal()))
				}
			}
			// Ensure all 4 expected states were present
			require.Equal(t, 4, len(present))
		default:
			require.Nil(t, m.Name(), fmt.Sprintf("metric %s not expected", m.Name()))
		}

	}
}
