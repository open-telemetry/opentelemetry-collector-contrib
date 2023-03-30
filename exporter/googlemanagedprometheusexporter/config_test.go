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

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 3)

	r0 := cfg.Exporters[component.NewID(typeStr)].(*Config)
	assert.Equal(t, r0, factory.CreateDefaultConfig().(*Config))

	r1 := cfg.Exporters[component.NewIDWithName(typeStr, "customname")].(*Config)
	assert.Equal(t, r1,
		&Config{
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 20 * time.Second,
			},
			GMPConfig: GMPConfig{
				ProjectID: "my-project",
				UserAgent: "opentelemetry-collector-contrib {{version}}",
			},
			RetrySettings: exporterhelper.RetrySettings{
				Enabled:             true,
				InitialInterval:     10 * time.Second,
				MaxInterval:         1 * time.Minute,
				MaxElapsedTime:      10 * time.Minute,
				RandomizationFactor: backoff.DefaultRandomizationFactor,
				Multiplier:          backoff.DefaultMultiplier,
			},
			QueueSettings: exporterhelper.QueueSettings{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
		})

	r2 := cfg.Exporters[component.NewIDWithName(typeStr, "customprefix")].(*Config)
	r2Expected := factory.CreateDefaultConfig().(*Config)
	r2Expected.GMPConfig.MetricConfig.Prefix = "my-metric-domain.com"
	assert.Equal(t, r2, r2Expected)
}
