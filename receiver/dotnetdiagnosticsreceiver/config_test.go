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

package dotnetdiagnosticsreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factories.Receivers[typeStr] = NewFactory()
	collectorCfg, err := configtest.LoadConfigAndValidate(path.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, collectorCfg)

	cfg := collectorCfg.Receivers[config.NewID(typeStr)].(*Config)
	require.NotNil(t, cfg)

	assert.Equal(t, 1234, cfg.PID)
	assert.Equal(t, 2*time.Second, cfg.CollectionInterval)
	assert.Equal(t, []string{"Foo", "Bar"}, cfg.Counters)
}
