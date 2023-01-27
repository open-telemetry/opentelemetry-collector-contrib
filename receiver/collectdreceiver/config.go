// Copyright 2019, OpenTelemetry Authors
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

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"
)

// Config defines configuration for Collectd receiver.
type Config struct {
	confignet.TCPAddr `mapstructure:",squash"`

	Timeout          time.Duration `mapstructure:"timeout"`
	AttributesPrefix string        `mapstructure:"attributes_prefix"`
	Encoding         string        `mapstructure:"encoding"`
}

func (cfg *Config) Unmarshal(parser *confmap.Conf) error {
	if parser == nil {
		return nil
	}
	err := parser.Unmarshal(cfg) // , confmap.WithErrorUnused()) // , cmpopts.IgnoreUnexported(metadata.MetricSettings{}))
	if err != nil {
		return err
	}
	return nil
}
