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

package datadogexporter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

func TestNewExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	cfg := &Config{
		API: APIConfig{
			Key: "ddog_32_characters_long_api_key1",
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:         HistogramModeDistributions,
				SendCountSum: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
		},
	}
	params := componenttest.NewNopExporterCreateSettings()
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetricsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	err = exp.ConsumeMetrics(context.Background(), testutils.TestMetrics.Clone())
	require.NoError(t, err)
	assert.Equal(t, len(server.MetadataChan), 0)

	cfg.HostMetadata.Enabled = true
	cfg.HostMetadata.HostnameSource = HostnameSourceFirstResource
	err = exp.ConsumeMetrics(context.Background(), testutils.TestMetrics.Clone())
	require.NoError(t, err)
	body := <-server.MetadataChan
	var recvMetadata metadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
}
