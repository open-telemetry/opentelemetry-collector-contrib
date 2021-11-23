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

package system

import "fmt"

// Config defines user-specified configurations unique to the system detector
type Config struct {
	// The HostnameSources is a priority list of sources from which hostname will be fetched.
	// In case of the error in fetching hostname from source,
	// the next source from the list will be considered.(**default**: `["dns", "os"]`)
	HostnameSources []string `mapstructure:"hostname_sources"`
}

// Validate config
func (cfg *Config) Validate() error {
	for _, hostnameSource := range cfg.HostnameSources {
		_, exists := hostnameSourcesMap[hostnameSource]
		if !exists {
			return fmt.Errorf("hostname_sources contains invalid value: %q", hostnameSource)
		}
	}
	return nil
}
