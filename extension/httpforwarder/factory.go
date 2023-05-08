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

package httpforwarder // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder/internal/metadata"
)

const (
	// The value of extension "type" in configuration.
	typeStr component.Type = "http_forwarder"

	// Default endpoints to bind to.
	defaultEndpoint = ":6060"
)

// NewFactory creates a factory for HostObserver extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability)
}

func createDefaultConfig() component.Config {
	return &Config{
		Ingress: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
		Egress: confighttp.HTTPClientSettings{
			Timeout: 10 * time.Second,
		},
	}
}

func createExtension(
	_ context.Context,
	params extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	return newHTTPForwarder(cfg.(*Config), params.TelemetrySettings)
}
