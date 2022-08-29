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

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "coralogix"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// Config defines by Coralogix.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`

	// Coralogix traces ingress endpoint
	Traces configgrpc.GRPCClientSettings `mapstructure:"traces"`

	// The Coralogix metrics ingress endpoint
	Metrics configgrpc.GRPCClientSettings `mapstructure:"metrics"`

	// The Coralogix logs ingress endpoint
	Logs configgrpc.GRPCClientSettings `mapstructure:"logs"`

	// Your Coralogix private key (sensitive) for authentication
	PrivateKey string `mapstructure:"private_key"`

	// Traces emitted by this OpenTelemetry exporter should be tagged
	// in Coralogix with the following application and subsystem names
	AppName string `mapstructure:"application_name"`

	// Deprecated: [v0.47.0] SubSystem will remove in the next version
	// You can remove 'subsystem_name' from your config file or leave it in this version.
	// The subsystem will generate automatically according to the "service_name" of the trace batch.
	SubSystem string `mapstructure:"subsystem_name"`
}

func isEmpty(endpoint string) bool {
	if endpoint == "" || endpoint == "https://" || endpoint == "http://" {
		return true
	}
	return false
}
func (c *Config) Validate() error {
	// validate that at least one endpoint is set up correctly
	if isEmpty(c.Traces.Endpoint) &&
		isEmpty(c.Metrics.Endpoint) &&
		isEmpty(c.Logs.Endpoint) {
		return fmt.Errorf("`traces.endpoint` or `metrics.endpoint` or `logs.endpoint` not specified, please fix the configuration file")
	}
	if c.PrivateKey == "" {
		return fmt.Errorf("`privateKey` not specified, please fix the configuration file")
	}
	if c.AppName == "" {
		return fmt.Errorf("`appName` not specified, please fix the configuration file")
	}

	return nil
}
