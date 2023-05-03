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

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "influxdb"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// V1Compatibility is used to specify if the exporter should use the v1.X InfluxDB API schema.
type V1Compatibility struct {
	// Enabled is used to specify if the exporter should use the v1.X InfluxDB API schema
	Enabled bool `mapstructure:"enabled"`
	// DB is used to specify the name of the V1 InfluxDB database that telemetry will be written to.
	DB string `mapstructure:"db"`
	// Username is used to optionally specify the basic auth username
	Username string `mapstructure:"username"`
	// Password is used to optionally specify the basic auth password
	Password configopaque.String `mapstructure:"password"`
}

// Config defines configuration for the InfluxDB exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// Org is the InfluxDB organization name of the destination bucket.
	Org string `mapstructure:"org"`
	// Bucket is the InfluxDB bucket name that telemetry will be written to.
	Bucket string `mapstructure:"bucket"`
	// Token is used to identify InfluxDB permissions within the organization.
	Token configopaque.String `mapstructure:"token"`
	// V1Compatibility is used to specify if the exporter should use the v1.X InfluxDB API schema.
	V1Compatibility V1Compatibility `mapstructure:"v1_compatibility"`

	// MetricsSchema indicates the metrics schema to emit to line protocol.
	// Options:
	// - telegraf-prometheus-v1
	// - telegraf-prometheus-v2
	MetricsSchema string `mapstructure:"metrics_schema"`
}

func (cfg *Config) Validate() error {
	return nil
}
