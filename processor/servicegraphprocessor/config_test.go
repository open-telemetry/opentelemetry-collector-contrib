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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	// Prepare
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factories.Processors[typeStr] = NewFactory()

	// Test
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "service-graph-config.yaml"), factories)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t,
		&Config{
			ProcessorSettings:       config.NewProcessorSettings(config.NewComponentID(typeStr)),
			MetricsExporter:         "metrics",
			LatencyHistogramBuckets: []time.Duration{1, 2, 3, 4, 5},
			Dimensions:              []string{"dimension-1", "dimension-2"},
			Store: StoreConfig{
				TTL:      time.Second,
				MaxItems: 10,
			},
		},
		cfg.Processors[config.NewComponentID(typeStr)],
	)
}
