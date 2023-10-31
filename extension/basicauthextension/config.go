// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"errors"

	"go.opentelemetry.io/collector/config/configopaque"
)

var (
	errNoCredentialSource     = errors.New("no credential source provided")
	errMultipleAuthenticators = errors.New("only one of `htpasswd` or `client_auth` can be specified")
)

type HtpasswdSettings struct {
	// Path to the htpasswd file.
	File string `mapstructure:"file"`
	// Inline contents of the htpasswd file.
	Inline string `mapstructure:"inline"`
}

type ClientAuthSettings struct {
	// Username holds the username to use for client authentication.
	Username string `mapstructure:"username"`
	// Password holds the password to use for client authentication.
	Password configopaque.String `mapstructure:"password"`
}
type Config struct {

	// Htpasswd settings.
	Htpasswd *HtpasswdSettings `mapstructure:"htpasswd,omitempty"`

	// ClientAuth settings
	ClientAuth *ClientAuthSettings `mapstructure:"client_auth,omitempty"`
}

func (cfg *Config) Validate() error {
	serverCondition := cfg.Htpasswd != nil
	clientCondition := cfg.ClientAuth != nil

	if serverCondition && clientCondition {
		return errMultipleAuthenticators
	}

	if !serverCondition && !clientCondition {
		return errNoCredentialSource
	}

	return nil
}
