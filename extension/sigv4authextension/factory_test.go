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

package sigv4authextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)

	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, f.CreateDefaultConfig().(*Config), cfg)

	ext, _ := createExtension(context.Background(), componenttest.NewNopExtensionCreateSettings(), cfg)
	fext, _ := f.CreateExtension(context.Background(), componenttest.NewNopExtensionCreateSettings(), cfg)
	assert.Equal(t, fext, ext)
}

func TestCreateDefaultConfig(t *testing.T) {
	expectedExtensionSettings := config.NewExtensionSettings(config.NewComponentID(typeStr))
	expectedComponentID := expectedExtensionSettings.ID()

	cfg := createDefaultConfig().(*Config)

	assert.Equal(t, expectedExtensionSettings, cfg.ExtensionSettings)
	assert.Equal(t, expectedComponentID, cfg.ExtensionSettings.ID())
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestCreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	ext, err := createExtension(context.Background(), componenttest.NewNopExtensionCreateSettings(), cfg)
	assert.Nil(t, err)
	assert.NotNil(t, ext)

}
