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

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"fmt"

	"go.opentelemetry.io/collector/config"
)

var (
	errMissingHeader        = fmt.Errorf("missing header name")
	errMissingHeadersConfig = fmt.Errorf("missing headers configuration")
	errMissingSource        = fmt.Errorf("missing header source, must be 'from_context' or 'value'")
	errConflictingSources   = fmt.Errorf("invalid header source, must either 'from_context' or 'value'")
)

type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`
	HeadersConfig            []HeaderConfig `mapstructure:"headers"`
}

type HeaderConfig struct {
	Key         *string `mapstructure:"key"`
	Value       *string `mapstructure:"value"`
	FromContext *string `mapstructure:"from_context"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.HeadersConfig == nil || len(cfg.HeadersConfig) == 0 {
		return errMissingHeadersConfig
	}
	for _, header := range cfg.HeadersConfig {
		if header.Key == nil || *header.Key == "" {
			return errMissingHeader
		}
		if header.FromContext == nil && header.Value == nil {
			return errMissingSource
		}
		if header.FromContext != nil && header.Value != nil {
			return errConflictingSources
		}
	}
	return nil
}
