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

package hostobserver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
)

const (
	// The value of extension "type" in configuration.
	typeStr configmodels.Type = "host_observer"

	defaultCollectionInterval = 10
)

// Factory is the factory for the extension.
type Factory struct {
}

var _ component.Factory = (*Factory)(nil)

// Type gets the type of the config created by this factory.
func (f *Factory) Type() configmodels.Type {
	return typeStr
}

// CreateDefaultConfig creates the default configuration for the extension.
func (f *Factory) CreateDefaultConfig() configmodels.Extension {
	return &Config{
		ExtensionSettings: configmodels.ExtensionSettings{
			TypeVal: typeStr,
			NameVal: string(typeStr),
		},
		RefreshInterval: defaultCollectionInterval * time.Second,
	}
}

// CreateExtension creates the extension based on this config.
func (f *Factory) CreateExtension(
	ctx context.Context,
	params component.ExtensionCreateParams,
	cfg configmodels.Extension,
) (component.ServiceExtension, error) {
	config := cfg.(*Config)
	return newObserver(params.Logger, config)
}
