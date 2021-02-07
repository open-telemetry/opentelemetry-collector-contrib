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

package gcloudpubsubreceiver

import (
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`

	ProjectID string `mapstructure:"project"`
	UserAgent string `mapstructure:"user_agent"`
	Endpoint  string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	UseInsecure bool `mapstructure:"use_insecure"`
	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	ValidateExistence   bool   `mapstructure:"validate_existence"`
	MetricsSubscription string `mapstructure:"metrics_subscription"`
	TracesSubscription  string `mapstructure:"traces_subscription"`
	LogsSubscription    string `mapstructure:"logs_subscription"`

	ClientId string `mapstructure:"client_id"`
}
