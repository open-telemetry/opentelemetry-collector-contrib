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

package datadogexporter

import "os"

// GetHost gets the hostname according to configuration.
// It gets the configuration hostname and if
// not available it relies on the OS hostname
func GetHost(cfg *Config) *string {
	if cfg.TagsConfig.Hostname != "" {
		return &cfg.TagsConfig.Hostname
	}

	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return &host
}
