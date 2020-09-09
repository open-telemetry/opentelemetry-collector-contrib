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

package httpforwarder

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration for http forwarder extension.
type Config struct {
	configmodels.ExtensionSettings `mapstructure:",squash"`
	confighttp.HTTPServerSettings  `mapstructure:",squash"`

	// URL to forward incoming HTTP requests to.
	ForwardTo string `mapstructure:"forward_to"`

	// Headers to add to all requests that pass through the forwarder.
	Headers map[string]string `mapstructure:"headers"`

	// HTTPTimeout is the timeout for forwarding HTTP request. The default value
	// is 10 seconds.
	HTTPTimeout time.Duration `mapstructure:"http_timeout"`
}
