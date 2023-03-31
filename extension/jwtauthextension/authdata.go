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
)

var _ client.AuthData = (*authData)(nil)

type authData struct {
	jwtClaims jwt.MapClaims
}

func (a *authData) GetAttribute(name string) interface{} {
	if val, ok := a.jwtClaims[name]; ok {
		return val
	}
	return nil
}

func (a *authData) GetAttributeNames() []string {
	keys := make([]string, 0, len(a.jwtClaims))

	for k := range a.jwtClaims {
		keys = append(keys, k)
	}

	return keys
}
