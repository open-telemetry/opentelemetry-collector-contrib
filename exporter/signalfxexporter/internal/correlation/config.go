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

package correlation

import (
	"errors"
	"net/url"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"go.opentelemetry.io/collector/config/confighttp"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

// DefaultConfig returns default configuration correlation values.
func DefaultConfig() *Config {
	return &Config{
		HTTPClientSettings:  confighttp.HTTPClientSettings{Timeout: 5 * time.Second},
		StaleServiceTimeout: 5 * time.Minute,
		SyncAttributes: map[string]string{
			conventions.AttributeK8SPodUID:   conventions.AttributeK8SPodUID,
			conventions.AttributeContainerID: conventions.AttributeContainerID,
		},
		Config: correlations.Config{
			MaxRequests:     20,
			MaxBuffered:     10_000,
			MaxRetries:      2,
			LogUpdates:      false,
			RetryDelay:      30 * time.Second,
			CleanupInterval: 1 * time.Minute,
		},
	}
}

// Config defines configuration for correlation via traces.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	correlations.Config           `mapstructure:",squash"`

	// How long to wait after a trace span's service name is last seen before
	// uncorrelating that service.
	StaleServiceTimeout time.Duration `mapstructure:"stale_service_timeout"`
	// SyncAttributes is a key of the span attribute name to sync to the dimension as the value.
	SyncAttributes map[string]string `mapstructure:"sync_attributes"`
}

func (c *Config) validate() error {
	if c.Endpoint == "" {
		return errors.New("`correlation.endpoint` not specified")
	}

	_, err := url.Parse(c.Endpoint)
	if err != nil {
		return err
	}

	return nil
}
