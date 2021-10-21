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

package cloudfoundryreceiver

import (
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for Collectd receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`

	RLPGatewayURL           string        `mapstructure:"rlp_gateway_url"`
	RLPGatewaySkipTLSVerify bool          `mapstructure:"rlp_gateway_skip_tls_verify"`
	RLPGatewayShardID       string        `mapstructure:"rlp_gateway_shard_id"`
	UAAUrl                  string        `mapstructure:"uaa_url"`
	UAASkipTLSVerify        bool          `mapstructure:"uaa_skip_tls_verify"`
	UAAUsername             string        `mapstructure:"uaa_username"`
	UAAPassword             string        `mapstructure:"uaa_password"`
	HTTPTimeout             time.Duration `mapstructure:"http_timeout"`
}

func (c *Config) Validate() error {
	err := validateURLOption("rlp_gateway_url", c.RLPGatewayURL)
	if err != nil {
		return err
	}

	err = validateURLOption("uaa_url", c.UAAUrl)
	if err != nil {
		return err
	}

	if c.UAAUsername == "" {
		return fmt.Errorf("username not specified")
	}

	return nil
}

func validateURLOption(name string, value string) error {
	if value == "" {
		return fmt.Errorf("%s not specified", name)
	}

	_, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("failed to parse %s as url: %v", name, err)
	}

	return nil
}
