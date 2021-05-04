// Copyright 2021, OpenTelemetry Authors
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

package influxdbexporter

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "influxdb"
)

// Config defines configuration for the InfluxDB exporter.
type Config struct {
	config.ExporterSettings       `mapstructure:",squash"`
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// Org is the InfluxDB organization name of the destination bucket.
	Org string `mapstructure:"org"`
	// Bucket is the InfluxDB bucket name that telemetry will be written to.
	Bucket string `mapstructure:"bucket"`
	// Token is used to identify InfluxDB permissions within the organization.
	Token string `mapstructure:"token"`

	// MetricsSchema indicates the metrics schema to emit to line protocol.
	// Options:
	// - telegraf-prometheus-v1
	// - telegraf-prometheus-v2
	MetricsSchema string `mapstructure:"metrics_schema"`
}
