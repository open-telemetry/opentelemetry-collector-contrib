// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsproxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

const (
	// The value of extension "type" in configuration.
	typeStr = "awsproxy"

	defaultEndpoint = "0.0.0.0:2000"
)

// NewFactory creates a factory for awsproxy extension.
func NewFactory() component.ExtensionFactory {
	return component.NewExtensionFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
	)
}

func createDefaultConfig() config.Extension {
	return &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
		ProxyConfig: proxy.Config{
			TCPAddr: confignet.TCPAddr{
				Endpoint: defaultEndpoint,
			},
			TLSSetting: configtls.TLSClientSetting{
				Insecure: false,
			},
		},
	}
}

func createExtension(_ context.Context, params component.ExtensionCreateSettings, cfg config.Extension) (component.Extension, error) {
	return newXrayProxy(cfg.(*Config), params.Logger)
}
