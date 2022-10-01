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

package resourcedetectionprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	lp, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)
}

func TestCreateConfigProcessors(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	for k := range cm.ToStringMap() {
		// Check if all processor variations that are defined in test config can be actually created
		t.Run(k, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(k)
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			tt, err := factory.CreateTracesProcessor(
				context.Background(),
				componenttest.NewNopProcessorCreateSettings(),
				cfg,
				consumertest.NewNop(),
			)
			assert.NoError(t, err)
			assert.NotNil(t, tt)
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"not-existing"}

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, tp)

	mp, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, mp)

	lp, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, lp)
}
