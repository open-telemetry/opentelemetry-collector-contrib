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

package jwtauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/jwtauthextension"

// Config has the configuration for the OIDC Authenticator extension.
type Config struct {

	// The attribute (header name) to look for auth data. Optional, default value: "authorization".
	Attribute string `mapstructure:"attribute"`

	// The local path for the issuer CA's TLS server cert.
	// Optional.
	JWTSecret string `mapstructure:"secret"`
}
