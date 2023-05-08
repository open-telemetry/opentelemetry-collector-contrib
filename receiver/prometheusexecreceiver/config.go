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

package prometheusexecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver"

import (
	"errors"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

// Config definition for prometheus_exec configuration
type Config struct {
	// Generic receiver config
	// ScrapeInterval is the time between each scrape completed by the Receiver
	ScrapeInterval time.Duration `mapstructure:"scrape_interval,omitempty"`
	// ScrapeTimeout is the time to wait before throttling a scrape request
	ScrapeTimeout time.Duration `mapstructure:"scrape_timeout,omitempty"`
	// Port is the port assigned to the Receiver, and to the {{port}} template variables
	Port int `mapstructure:"port"`
	// SubprocessConfig is the configuration needed for the subprocess
	SubprocessConfig subprocessmanager.SubprocessConfig `mapstructure:",squash"`
}

func (cfg *Config) Validate() error {
	if cfg.SubprocessConfig.Command == "" {
		return errors.New("command to execute must be non-empty")
	}
	return nil
}
