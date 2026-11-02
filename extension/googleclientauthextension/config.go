// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googleclientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"
)

// Config defines configuration for the Google client auth extension.
type Config struct {
	Config clientauth.Config `mapstructure:",squash"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	if err := cfg.Config.Validate(); err != nil {
		return fmt.Errorf("googleclientauth settings are invalid :%w", err)
	}
	return nil
}
