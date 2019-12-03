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

package signalfxexporter

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
)

// Config defines configuration for SignalFx exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// AccessToken is the authentication token provided by SignalFx.
	AccessToken string `mapstructure:"access_token"`

	// Realm is the SignalFx realm where data is going to be sent to. The
	// default value is "us0"
	Realm string `mapstructure:"realm"`

	// URL is the destination to where SignalFx metrics will be sent to, it is
	// intended for tests and debugging. The value of Realm is ignored if the
	// URL is specified. If a path is not included the exporter will
	// automatically append the appropriate path, eg.: "v2/datapoint".
	// If a path is specified it will use the one set by the config.
	URL string `mapstructure:"url"`

	// Timeout is the maximum timeout for HTTP request sending trace data. The
	// default value is 5 seconds.
	Timeout time.Duration `mapstructure:"timeout"`

	// Headers are a set of headers to be added to the HTTP request sending
	// trace data. These can override pre-defined header values used by the
	// exporter, eg: "User-Agent" can be set to a custom value if specified
	// here.
	Headers map[string]string `mapstructure:"headers"`
}
