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

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"errors"

	"github.com/oklog/ulid/v2"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config has the configuration for the opamp extension.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`

	// InstanceUID is a ULID formatted as a 26 character string in canonical
	// representation. Auto-generated on start if missing.
	InstanceUID string `mapstructure:"instance_uid"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.InstanceUID != "" {
		_, err := ulid.ParseStrict(cfg.InstanceUID)
		if err != nil {
			return errors.New("opamp instance_uid is invalid")
		}
	}
	return nil
}
