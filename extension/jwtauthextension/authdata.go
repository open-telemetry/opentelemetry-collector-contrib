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

import (
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/collector/client"
	"strings"
)

var _ client.AuthData = (*authData)(nil)

type authData struct {
	issuer    string
	subject   string
	audience  []string
	jwtClaims jwt.MapClaims
}

func (a *authData) GetAttribute(name string) interface{} {
	switch name {
	case "issuer":
		return a.issuer
	case "subject":
		return a.subject
	case "audience":
		return a.audience
	case "jwtClaims":
		return a.jwtClaims
	default:
		segments := strings.Split(name, ".")
		if strings.HasPrefix(name, "jwtClaims.") && len(segments) == 2 {
			if val, ok := a.jwtClaims[segments[1]]; ok {
				return val
			}
		}
		return nil
	}
}

func (*authData) GetAttributeNames() []string {
	return []string{"issuer", "subject", "audience", "jwtClaims"}
}
