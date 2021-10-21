// Copyright  OpenTelemetry Authors
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

package ecsobserver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/extensionhelper"
)

const (
	typeStr config.Type = "ecs_observer"
)

// NewFactory creates a factory for ECSObserver extension.
func NewFactory() component.ExtensionFactory {
	return extensionhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		createExtension,
	)
}

func createDefaultConfig() config.Extension {
	cfg := DefaultConfig()
	return &cfg
}

func createExtension(ctx context.Context, params component.ExtensionCreateSettings, cfg config.Extension) (component.Extension, error) {
	sdCfg := cfg.(*Config)
	fetcher, err := newTaskFetcherFromConfig(*sdCfg, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("init fetcher failed: %w", err)
	}
	return createExtensionWithFetcher(params, sdCfg, fetcher)
}

// fetcher is mock in unit test or AWS API client
func createExtensionWithFetcher(params component.ExtensionCreateSettings, sdCfg *Config, fetcher *taskFetcher) (component.Extension, error) {
	sd, err := newDiscovery(*sdCfg, serviceDiscoveryOptions{Logger: params.Logger, Fetcher: fetcher})
	if err != nil {
		return nil, err
	}
	return &ecsObserver{
		logger: params.Logger,
		sd:     sd,
	}, nil
}
