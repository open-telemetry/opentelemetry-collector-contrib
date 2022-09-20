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

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sqlquery"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/service/servicetest"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
)

func TestParseConfig(t *testing.T) {
	cfg, err := servicetest.LoadConfigAndValidate(path.Join("testdata", "config.yaml"), testFactories(t))
	require.NoError(t, err)
	sqlCfg := cfg.Receivers[config.NewComponentID("sqlquery")].(*Config)
	assert.Equal(t, "mydriver", sqlCfg.Driver)
	assert.Equal(t, "host=localhost port=5432 user=me password=s3cr3t sslmode=disable", sqlCfg.DataSource)
	q := sqlCfg.Queries[0]
	assert.Equal(t, "select count(*) as count, type from mytable group by type", q.SQL)
	metric := q.Metrics[0]
	assert.Equal(t, "val.count", metric.MetricName)
	assert.Equal(t, "count", metric.ValueColumn)
	assert.Equal(t, "type", metric.AttributeColumns[0])
	assert.Equal(t, false, metric.Monotonic)
	assert.Equal(t, MetricTypeSum, metric.DataType)
	assert.Equal(t, MetricValueTypeInt, metric.ValueType)
	assert.Equal(t, map[string]string{"foo": "bar"}, metric.StaticAttributes)
	assert.Equal(t, MetricAggregationCumulative, metric.Aggregation)
}

func testFactories(t *testing.T) component.Factories {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factories.Receivers["sqlquery"] = component.NewReceiverFactory(
		"sqlquery",
		func() config.Receiver {
			return CreateDefaultConfig("sqlquery")
		},
	)
	return factories
}
