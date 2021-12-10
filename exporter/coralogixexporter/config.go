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

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typestr = "coralogix"
)

// Config defines by Coralogix.
type Config struct {
	config.ExporterSettings      `mapstructure:",squash"`
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	// The Coralogix logs ingress endpoint
	Endpoint string `mapstructure:"endpoint"`

	// Your Coralogix private key (sensitive) for authentication
	PrivateKey string `mapstructure:"private_key"`

	// Traces emitted by this OpenTelemetry exporter should be tagged
	// in Coralogix with the following application and subsystem names
	AppName   string `mapstructure:"application_name"`
	SubSystem string `mapstructure:"subsystem_name"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" || c.PrivateKey == "" || c.AppName == "" || c.SubSystem == "" {
		return fmt.Errorf("One of your configureation field (endpoint, private_key, application_name, subsystem_name) is empty, please fix the configuration file")
	}
	return nil
}
