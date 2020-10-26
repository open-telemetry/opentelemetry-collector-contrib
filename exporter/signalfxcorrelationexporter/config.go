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

package signalfxcorrelationexporter

import (
	"errors"
	"net/url"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration for signalfx_correlation exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	correlations.Config           `mapstructure:",squash"`

	// How long to wait after a trace span's service name is last seen before
	// uncorrelating that service.
	StaleServiceTimeout time.Duration `mapstructure:"stale_service_timeout"`
	// SyncAttributes is a key of the span attribute name to sync to the dimension as the value.
	SyncAttributes map[string]string `mapstructure:"sync_attributes"`

	// AccessToken is the authentication token provided by SignalFx.
	AccessToken string `mapstructure:"access_token"`
}

func (c *Config) validate() error {
	if c.Endpoint == "" {
		return errors.New("`endpoint` not specified")
	}

	_, err := url.Parse(c.Endpoint)
	if err != nil {
		return err
	}

	return nil
}
