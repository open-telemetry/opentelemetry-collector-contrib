// Copyright The OpenTelemetry Authors
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

package apachereceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.EqualValues(t, "apache", ft)
}

func TestValidConfig(t *testing.T) {
	factory := NewFactory()
	require.NoError(t, component.ValidateConfig(factory.CreateDefaultConfig()))
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	metricsReceiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		&Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: 10 * time.Second,
			},
		},
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)
}

func TestPortValidate(t *testing.T) {
	testCases := []struct {
		desc         string
		endpoint     string
		expectedPort string
	}{
		{
			desc:         "http with specified port",
			endpoint:     "http://localhost:8080/server-status?auto",
			expectedPort: "8080",
		},
		{
			desc:         "http without specified port",
			endpoint:     "http://localhost/server-status?auto",
			expectedPort: "80",
		},
		{
			desc:         "https with specified port",
			endpoint:     "https://localhost:8080/server-status?auto",
			expectedPort: "8080",
		},
		{
			desc:         "https without specified port",
			endpoint:     "https://localhost/server-status?auto",
			expectedPort: "443",
		},
		{
			desc:         "unknown protocol with specified port",
			endpoint:     "abc://localhost:8080/server-status?auto",
			expectedPort: "8080",
		},
		{
			desc:         "port unresolvable",
			endpoint:     "abc://localhost/server-status?auto",
			expectedPort: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.Endpoint = tc.endpoint
			_, port, err := parseResourceAttributes(tc.endpoint)

			require.NoError(t, err)
			require.Equal(t, tc.expectedPort, port)
		})
	}
}
