// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecstaskobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver"

import (
	"context"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

// NewFactory creates a factory for ECSTaskObserver extension.
func NewFactory() extension.Factory {
	return extension.NewFactory(
		metadata.Type,
		createDefaultConfig,
		createExtension,
		metadata.ExtensionStability,
	)
}

func createDefaultConfig() component.Config {
	cfg := defaultConfig()
	return &cfg
}

type baseExtension struct {
	component.StartFunc
	component.ShutdownFunc
}

func createExtension(
	_ context.Context,
	params extension.CreateSettings,
	cfg component.Config,
) (extension.Extension, error) {
	obsCfg := cfg.(*Config)

	var metadataProvider ecsutil.MetadataProvider
	var err error
	if obsCfg.Endpoint == "" {
		metadataProvider, err = ecsutil.NewDetectedTaskMetadataProvider(params.TelemetrySettings)
	} else {
		metadataProvider, err = metadataProviderFromEndpoint(obsCfg, params.TelemetrySettings)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create ECS Task Observer metadata provider: %w", err)
	}

	e := &ecsTaskObserver{
		config:           obsCfg,
		metadataProvider: metadataProvider,
		telemetry:        params.TelemetrySettings,
	}
	e.Extension = baseExtension{
		ShutdownFunc: e.Shutdown,
	}
	e.EndpointsWatcher = observer.NewEndpointsWatcher(e, obsCfg.RefreshInterval, params.TelemetrySettings.Logger)

	return e, nil
}

func metadataProviderFromEndpoint(config *Config, settings component.TelemetrySettings) (ecsutil.MetadataProvider, error) {
	parsed, err := url.Parse(config.Endpoint)
	if err != nil || parsed == nil {
		return nil, fmt.Errorf("failed to parse task metadata endpoint: %w", err)
	}

	restClient, err := ecsutil.NewRestClient(*parsed, config.HTTPClientSettings, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create ECS Task Observer rest client: %w", err)
	}

	return ecsutil.NewTaskMetadataProvider(restClient, settings.Logger), nil
}
