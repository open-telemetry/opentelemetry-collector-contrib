// Copyright  The OpenTelemetry Authors
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

package activedirectorydsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, len(cfg.Receivers), 2)
	defaultRecvID := config.NewComponentIDWithName(typeStr, "defaults")

	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.ReceiverSettings.SetIDName(defaultRecvID.Name())
	defaultReceiver := cfg.Receivers[defaultRecvID]
	require.Equal(t, defaultCfg, defaultReceiver)

	advancedRecv := cfg.Receivers[config.NewComponentID(typeStr)]
	expectedAdvancedRecv := factory.CreateDefaultConfig().(*Config)

	expectedAdvancedRecv.Metrics.ActiveDirectoryDsReplicationObjectRate.Enabled = false
	expectedAdvancedRecv.ScraperControllerSettings.CollectionInterval = 2 * time.Minute

	require.Equal(t, expectedAdvancedRecv, advancedRecv)
}
