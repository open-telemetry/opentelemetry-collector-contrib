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

package datadogprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration options for datadogprocessor.
type Config struct {

	// MetricsExporter specifies the name of the metrics exporter to be used when
	// exporting stats metrics.
	MetricsExporter component.ID `mapstructure:"metrics_exporter"`
}

func createDefaultConfig() component.Config {
	return &Config{
		MetricsExporter: datadogComponent,
	}
}

// datadogComponent defines the default component that will be used for
// exporting metrics.
var datadogComponent = component.NewID(component.Type("datadog"))
