// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

var errEmptyEndpoint = errors.New("empty endpoint")

// Config defines configuration for the SignalFx receiver.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// Deprecated: `access_token_passthrough` is deprecated.
	// Please enable include_metadata in the receiver and add the following config to the batch processor:
	// batch:
	// 	 metadata_keys: [X-Sf-Token]
	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`
}

// Validate verifies that the endpoint is valid and the configured port is not 0
func (rCfg *Config) Validate() error {
	if rCfg.Endpoint == "" {
		return errEmptyEndpoint
	}

	_, err := extractPortFromEndpoint(rCfg.Endpoint)
	if err != nil {
		return err
	}
	return nil
}
