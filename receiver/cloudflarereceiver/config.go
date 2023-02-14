// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
)

var (
	defaultPollInterval = time.Minute
	// default count of 0 is to collect all log
	defaultCount = 0
	// default sample rate of 1.0 is to collect 100% of logs
	defaultSampleRate = 1.0
	// default fields are the default fields collected.
	defaultFields = []string{
		"ClientIP",
		"ClientRequestHost",
		"ClientRequestMethod",
		"ClientRequestURI",
		"EdgeEndTimestamp",
		"EdgeResponseBytes",
		"EdgeResponseStatus",
		"EdgeStartTimestamp",
		"RayID",
	}
)

// Config holds all the parameters to start and collect logs from Cloudflare
type Config struct {
	PollInterval time.Duration `mapstructure:"poll_interval"`
	Auth         *Auth         `mapstructure:"auth"`
	Zone         string        `mapstructure:"zone"`
	Logs         *LogsConfig   `mapstructure:"logs,omitempty"`
	StorageID    *component.ID `mapstructure:"storage"`
}

// Auth holds required credentials that are either a key and email combination or a token
type Auth struct {
	XAuthEmail string `mapstructure:"email,omitempty"`
	XAuthKey   string `mapstructure:"key,omitempty"`
	APIToken   string `mapstructure:"token,omitempty"`
}

// LogsConfig is the configuration for the logs portion of this receiver
type LogsConfig struct {
	Count  int      `mapstructure:"count,omitempty"`
	Sample float32  `mapstructure:"sample,omitempty"`
	Fields []string `mapstructure:"fields,omitempty"`
}

var (
	errNoZone                                 = errors.New("no zone was specified")
	errMissingRequiredFieldEdgeEndTimestamp   = errors.New("missing required field 'EdgeEndTimestamp'")
	errMissingRequiredFieldEdgeResponseStatus = errors.New("missing required field 'EdgeResponseStatus'")
	errInvalidCountConfigured                 = errors.New("count is improperly configured, value must be greater than 0")
	errInvalidSampleConfigured                = errors.New("sample is improperly configured, value must be between 0.001 and 1.0")
	errInvalidPollInterval                    = errors.New("poll interval is incorrect, it must be a duration greater than one second")
	errInvalidAuthenticationConfigured        = errors.New("auth is improperly configured, both email and key must be present or a token")
)

// Validate validates all portions of the relevant config
func (c *Config) Validate() error {
	var errs error
	if c.Zone == "" {
		errs = multierr.Append(errs, errNoZone)
	}
	if c.PollInterval < time.Second {
		errs = multierr.Append(errs, errInvalidPollInterval)
	}
	return multierr.Combine(errs, c.validateLogsConfig(), c.validateAuthConfig())
}

func (c *Config) validateAuthConfig() error {
	// Must have either an email and key or an API token for authentication
	if (c.Auth.XAuthEmail != "" && c.Auth.XAuthKey != "") || c.Auth.APIToken != "" {
		return nil
	}
	return errInvalidAuthenticationConfigured
}

func (c *Config) validateLogsConfig() error {
	if c.Logs.Count < 0 {
		return errInvalidCountConfigured
	}
	if c.Logs.Sample < 0.001 || c.Logs.Sample > 1.0 {
		return errInvalidSampleConfigured
	}
	if len(c.Logs.Fields) != 0 {
		return c.validateRequiredFields()
	}
	return nil
}

func (c *Config) validateRequiredFields() error {
	var errs error
	var foundTimestampField bool
	timestampField := "EdgeEndTimestamp"
	var foundStatusCodeField bool
	statusCode := "EdgeResponseStatus"

	for _, field := range c.Logs.Fields {
		if field == timestampField {
			foundTimestampField = true
		}
		if field == statusCode {
			foundStatusCodeField = true
		}
	}

	if !foundTimestampField {
		errs = multierr.Append(errs, errMissingRequiredFieldEdgeEndTimestamp)
	}

	if !foundStatusCodeField {
		errs = multierr.Append(errs, errMissingRequiredFieldEdgeResponseStatus)
	}

	return errs
}
