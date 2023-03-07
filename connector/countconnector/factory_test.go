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

package countconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), component.Type(typeStr))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		Traces: MetricInfo{
			Name:        defaultMetricNameSpans,
			Description: defaultMetricDescSpans,
		},
		Metrics: MetricInfo{
			Name:        defaultMetricNameDataPoints,
			Description: defaultMetricDescDataPoints,
		},
		Logs: MetricInfo{
			Name:        defaultMetricNameLogRecords,
			Description: defaultMetricDescLogRecords,
		},
	})
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}
