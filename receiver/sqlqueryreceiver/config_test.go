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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 10*time.Second, cfg.ScraperControllerSettings.CollectionInterval)
}

func TestParseConfig(t *testing.T) {
	cfg, err := servicetest.LoadConfigAndValidate(path.Join("testdata", "config.yaml"), testFactories(t))
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
	assert.Equal(t, false, metric.Monotonic)
	assert.Equal(t, MetricDataTypeGauge, metric.DataType)
	assert.Equal(t, MetricValueTypeInt, metric.ValueType)
	assert.Equal(t, MetricAggregationCumulative, metric.Aggregation)
}

func TestConfig_Validate_Invalid(t *testing.T) {
	cfgFiles := []string{
		"config-invalid-datatype.yaml",
		"config-invalid-valuetype.yaml",
		"config-invalid-aggregation.yaml",
		"config-invalid-missing-metricname.yaml",
		"config-invalid-missing-valuecolumn.yaml",
		"config-invalid-missing-sql.yaml",
		"config-invalid-missing-queries.yaml",
		"config-invalid-missing-driver.yaml",
		"config-invalid-missing-metrics.yaml",
		"config-invalid-missing-datasource.yaml",
	}
	for _, cfgFile := range cfgFiles {
		_, err := servicetest.LoadConfigAndValidate(
			path.Join("testdata", cfgFile),
			testFactories(t),
		)
		require.Error(t, err)
	}
}

func TestConfig_Validate_Multierr(t *testing.T) {
	_, err := servicetest.LoadConfigAndValidate(
		path.Join("testdata", "config-invalid-multierr.yaml"),
		testFactories(t),
	)
	assert.ErrorContains(t, err, "invalid metric config with metric_name 'my.metric'")
	assert.ErrorContains(t, err, "metric config has unsupported value_type: 'xint'")
	assert.ErrorContains(t, err, "metric config has unsupported data_type: 'xgauge'")
	assert.ErrorContains(t, err, "metric config has unsupported aggregation: 'xcumulative'")
}

func testFactories(t *testing.T) component.Factories {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factories.Receivers[typeStr] = NewFactory()
	return factories
}
