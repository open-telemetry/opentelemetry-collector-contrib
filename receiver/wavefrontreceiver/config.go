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

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confignet"
)

// Config defines configuration for the Wavefront receiver.
type Config struct {
	confignet.TCPAddr `mapstructure:",squash"`

	// TCPIdleTimeout is the timout for idle TCP connections.
	TCPIdleTimeout time.Duration `mapstructure:"tcp_idle_timeout"`

	// ExtractCollectdTags instructs the Wavefront receiver to attempt to extract
	// tags in the CollectD format from the metric name. The default is false.
	ExtractCollectdTags bool `mapstructure:"extract_collectd_tags"`
}
