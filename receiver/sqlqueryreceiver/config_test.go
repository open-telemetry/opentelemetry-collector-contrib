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

package sqlqueryreceiver

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	sqlCfg := cfg.(*Config)
	assert.Equal(t, 10*time.Second, sqlCfg.ScraperControllerSettings.CollectionInterval)
}

func TestParseConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factories.Receivers[typeStr] = NewFactory()
	cfg, err := servicetest.LoadConfigAndValidate(path.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	sqlCfg := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)
	assert.Equal(t, "mydriver", sqlCfg.Driver)
	assert.Equal(t, "host=localhost port=5432 user=me password=s3cr3t sslmode=disable", sqlCfg.DataSource)
	q := sqlCfg.Queries[0]
	assert.Equal(t, "select count(*) as count, type from mytable group by type", q.SQL)
	metric := q.Metrics[0]
	assert.Equal(t, "val.count", metric.MetricName)
	assert.Equal(t, "count", metric.ValueColumn)
	assert.Equal(t, "type", metric.AttributeColumns[0])
}
