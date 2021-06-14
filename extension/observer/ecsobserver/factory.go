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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/extension/extensionhelper"
)

type testOverrideKey string // need to define custom type to make linter happy

const (
	typeStr               config.Type     = "ecs_observer"
	ctxFetcherOverrideKey testOverrideKey = "fetcherOverride"
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
	opt := serviceDiscoveryOptions{Logger: params.Logger}
	// Only for test
	fetcher := ctx.Value(ctxFetcherOverrideKey)
	if fetcher != nil {
		opt.FetcherOverride = fetcher.(*taskFetcher)
	}
	sd, err := newDiscovery(*sdCfg, opt)
	if err != nil {
		return nil, err
	}
	return &ecsObserver{
		logger: params.Logger,
		sd:     sd,
	}, nil
}
