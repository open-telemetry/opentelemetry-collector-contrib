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

package awsprometheusremotewriteexporter

import (
	prw "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"
)

// Config defines configuration for Remote Write exporter.
type Config struct {
	// Config represents the Prometheus Remote Write Exporter configuration.
	prw.Config `mapstructure:",squash"`

	// AuthConfig represents the AWS SigV4 configuration options.
	AuthConfig AuthConfig `mapstructure:"aws_auth"`
}

// AuthConfig defines AWS authentication configurations for SigningRoundTripper.
type AuthConfig struct {
	// Region is the AWS region for AWS SigV4.
	Region string `mapstructure:"region"`

	// Service is the AWS service for AWS SigV4, this is by default "aps". Optional.
	Service string `mapstructure:"service"`

	// Amazon Resource Name (ARN) of a role to assume. Optional.
	RoleArn string `mapstructure:"role_arn"`
}
