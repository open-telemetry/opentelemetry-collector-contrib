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
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
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
	rms, err := sc.scrape(context.Background())
	require.Nil(t, err)

	require.Equal(t, 1, rms.Len())
	rm := rms.At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilms.Len())

	ilm := ilms.At(0)
	ms := ilm.Metrics()

	require.Equal(t, 7, ms.Len())

	metricValues := make(map[string]int64, 7)

	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)

		var dps pdata.IntDataPointSlice

		switch m.DataType() {
		case pdata.MetricDataTypeIntGauge:
			dps = m.IntGauge().DataPoints()
		case pdata.MetricDataTypeIntSum:
			dps = m.IntSum().DataPoints()
		}

		require.Equal(t, 1, dps.Len())

		metricValues[m.Name()] = dps.At(0).Value()
	}
}
