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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension"
)

func RetrieveBearerToken(authCfg configauth.Authentication, extensions map[component.ID]component.Component) (string, error) {
	auth, err := authCfg.GetClientAuthenticator(extensions)
	if err != nil {
		return "", err
	}

	// only bearer token auth is supported for now
	if _, ok := auth.(*bearertokenauthextension.BearerTokenAuth); !ok {
		return "", fmt.Errorf("only bearer token auth extension is supported at this moment, but got %T", auth)
	}

	cr, err := auth.PerRPCCredentials()
	if err != nil {
		return "", err
	}

	headers, err := cr.GetRequestMetadata(context.Background(), "")
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(headers["authorization"], "Bearer "), nil
}
