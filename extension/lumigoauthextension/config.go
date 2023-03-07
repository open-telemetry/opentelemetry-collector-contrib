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

package lumigoauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/lumigoauthextension"

import "fmt"

type Type string

const (
	Default Type = ""
	Server  Type = "server"
	Client  Type = "client"
)

type Config struct {
	Type  Type   `mapstructure:"type,omitempty"` // If empty, value equals to 'server'
	Token string `mapstructure:"token,omitempty"`
}

func (cfg *Config) Validate() error {
	switch cfg.Type {
	case Default:
		fallthrough
	case Server:
		if len(cfg.Token) > 0 {
			return fmt.Errorf("setting the 'token' configuration is not supported for 'type: server'")
		}
	case Client:
		if len(cfg.Token) > 0 {
			if err := ValidateLumigoToken(cfg.Token); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unrecognized type: '%s'", cfg.Type)
	}

	return nil
}
