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

package oracledbreceiver // import "github.com/open-telemetry/open-telemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := CreateDefaultConfig().(*Config)
	assert.Equal(t, 10*time.Second, cfg.ScraperControllerSettings.CollectionInterval)
}

func TestParseConfig(t *testing.T) {
	cfg, err := servicetest.LoadConfigAndValidate(path.Join("testdata", "config_test.yaml"), testFactories(t))
	require.NoError(t, err)
	sqlCfg := cfg.Receivers[component.NewID("oracledb")].(*Config)
	assert.Equal(t, "oracle://otel:password@localhost:51521/XE", sqlCfg.DataSource)
	settings := sqlCfg.MetricsSettings
	assert.False(t, settings.OracledbTablespaceSizeUsage.Enabled)
	assert.False(t, settings.OracledbExchangeDeadlocks.Enabled)
}

func testFactories(t *testing.T) component.Factories {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factories.Receivers["oracledb"] = component.NewReceiverFactory(
		"oracledb",
		CreateDefaultConfig,
	)
	return factories
}
