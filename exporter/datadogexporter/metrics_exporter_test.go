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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/testutils"
)

func TestNewExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	cfg := &config.Config{
		API: config.APIConfig{
			Key: "ddog_32_characters_long_api_key1",
		},
		Metrics: config.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
		},
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	// The client should have been created correctly
	exp := newMetricsExporter(context.Background(), params, cfg)
	assert.NotNil(t, exp)
	_, _ = exp.PushMetricsData(context.Background(), testutils.TestMetrics.Clone())
	assert.Equal(t, len(server.MetadataChan), 0)

	cfg.SendMetadata = true
	cfg.UseResourceMetadata = true
	_, _ = exp.PushMetricsData(context.Background(), testutils.TestMetrics.Clone())
	body := <-server.MetadataChan
	var recvMetadata metadata.HostMetadata
	err := json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
}
