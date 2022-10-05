// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"errors"

	"go.opentelemetry.io/collector/config"
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
	Password string `mapstructure:"password"`
}
type Config struct {
	config.ExtensionSettings `mapstructure:",squash"`

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
