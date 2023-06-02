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

package servicegraphprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
)

func TestLoadConfig(t *testing.T) {
	// Prepare
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factories.Processors[typeStr] = NewFactory()
	factories.Connectors[typeStr] = NewConnectorFactory()

	// Test
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "service-graph-config.yaml"), factories)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t,
		&Config{
			MetricsExporter:         "metrics",
			LatencyHistogramBuckets: []time.Duration{1, 2, 3, 4, 5},
			Dimensions:              []string{"dimension-1", "dimension-2"},
			Store: StoreConfig{
				TTL:      time.Second,
				MaxItems: 10,
			},
			CacheLoop:                 2 * time.Minute,
			StoreExpirationLoop:       10 * time.Second,
			VirtualNodePeerAttributes: []string{"db.name", "rpc.service"},
		},
		cfg.Processors[component.NewID(typeStr)],
	)

	cfg, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "service-graph-connector-config.yaml"), factories)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t,
		&Config{
			LatencyHistogramBuckets: []time.Duration{1, 2, 3, 4, 5},
			Dimensions:              []string{"dimension-1", "dimension-2"},
			Store: StoreConfig{
				TTL:      time.Second,
				MaxItems: 10,
			},
			CacheLoop:           time.Minute,
			StoreExpirationLoop: 2 * time.Second,
		},
		cfg.Connectors[component.NewID(typeStr)],
	)

}
