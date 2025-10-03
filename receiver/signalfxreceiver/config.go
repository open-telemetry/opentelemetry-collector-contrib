// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
)

var errEmptyEndpoint = errors.New("empty endpoint")

// Config defines configuration for the SignalFx receiver.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
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
