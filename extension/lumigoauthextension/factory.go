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

package lumigoauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/lumigoauthextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	typeStr = "lumigoauth"
)

// NewFactory creates a factory for the static bearer token Authenticator extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		component.StabilityLevelStable,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Type:  Server,
		Token: "",
	}
}

func createExtension(_ context.Context, set extension.CreateSettings, cfg component.Config) (extension.Extension, error) {
	// check if config is a server auth(Htpasswd should be set)
	switch cfg.(*Config).Type {
	case Default:
		fallthrough
	case Server:
		return newServerAuthExtension(cfg.(*Config), set.Logger)
	case Client:
		return newClientAuthExtension(cfg.(*Config), set.Logger)
	default:
		return nil, fmt.Errorf("unrecognized type: '%s'", cfg.(*Config).Type)
	}
}
