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

package sumologicexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, component.Type("sumologic"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	qs := exporterhelper.NewDefaultQueueSettings()
	qs.Enabled = false

	assert.Equal(t, cfg, &Config{
		CompressEncoding:   "gzip",
		MaxRequestBodySize: 1_048_576,
		LogFormat:          "json",
		MetricFormat:       "prometheus",
		SourceCategory:     "",
		SourceName:         "",
		SourceHost:         "",
		Client:             "otelcol",
		GraphiteTemplate:   "%{_metric_}",

		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: 5 * time.Second,
		},
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
		QueueSettings: qs,
	})
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}
