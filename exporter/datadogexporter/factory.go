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
package datadogexporter

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// typeStr is the type of the exporter
	typeStr = "datadog"

	// DefaultSite is the default site of the Datadog intake to send data to
	DefaultSite = "datadoghq.com"
)

// NewFactory creates a Datadog exporter factory
func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
	)
}

// createDefaultConfig creates the default exporter configuration
func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},

		TagsConfig: TagsConfig{
			Hostname: "${DD_HOST}",
			Env:      "${DD_ENV}",
			Service:  "${DD_SERVICE}",
			Version:  "${DD_VERSION}",
		},

		API: APIConfig{
			Key:  "", // must be set if using API
			Site: DefaultSite,
		},

		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "", // set during config sanitization
			},
		},

		Traces: TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "", // set during config sanitization
			},
		},
	}
}
