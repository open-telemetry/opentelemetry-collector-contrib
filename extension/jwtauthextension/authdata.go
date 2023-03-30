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

import "go.opentelemetry.io/collector/client"

var _ client.AuthData = (*authData)(nil)

type authData struct {
	issuer   string
	subject  string
	audience []string
}

func (a *authData) GetAttribute(name string) interface{} {
	switch name {
	case "issuer":
		return a.issuer
	case "subject":
		return a.subject
	case "audience":
		return a.audience
	default:
		return nil
	}
}

func (*authData) GetAttributeNames() []string {
	return []string{"issuer", "subject", "audience"}
}
