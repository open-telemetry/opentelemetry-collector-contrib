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

package dynamicconfig

import (
	"go.opentelemetry.io/collector/config/configmodels"
)

// Config has the configuration for the extension enabling the dynamic
// configuration service
type Config struct {
	configmodels.ExtensionSettings `mapstructure:",squash"`

	// Endpoint is the address and port used to communicate the config updates
	// to the SDK. The default value is localhost:55700.
	Endpoint string `mapstructure:"endpoint"`

	// RemoteConfigAddress is the address of an upstream remote configuration
	// service. If specified, the upstream remote configuration service will be
	// used as the source of config updates. Requests for config updates from
	// this collector will receive updates from the upstream configuration
	// service. If both "RemoteConfigAddress" and "LocalConfigFile" are
	// specified, then "RemoteConfigAddress" will take precedence.
	RemoteConfigAddress string `mapstructure:"remote_config_address"`

	// LocalConfigFile is the local record of configuration updates, applied
	// when a third-party config service backend is not used. If this file is
	// not specified, and no other config backends are specified, then it
	// defaults to the file "schedules.yaml", located in the same directory as
	// the collector-wide config. If both "LocalConfigFile" and
	// "RemoteConfigAddress" are specified, then "RemoteConfigAddress" takes
	// precedence.
	LocalConfigFile string `mapstructure:"local_config_file"`

	// WaitTime is the suggested time, in seconds, for the client to wait
	// before querying the collector for an updated configuration. Defaults to
	// 30 seconds.
	WaitTime int `mapstructure:"wait_time"`
}
