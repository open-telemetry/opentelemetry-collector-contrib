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

package filestorage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.Extensions, 2)

	ext0 := cfg.Extensions[config.NewComponentID(typeStr)]
	defaultConfig := factory.CreateDefaultConfig()
	defaultConfigFileStorage, ok := defaultConfig.(*Config)
	require.True(t, ok)
	// Specify a directory so that tests will pass because on some systems the
	// default dir (e.g. on not windows: /var/lib/otelcol/file_storage) might not
	// exist which will fail the test when config.Validate() will be called.
	defaultConfigFileStorage.Directory = "."
	assert.Equal(t, defaultConfig, ext0)

	ext1 := cfg.Extensions[config.NewComponentIDWithName(typeStr, "all_settings")]
	assert.Equal(t,
		&Config{
			ExtensionSettings: config.NewExtensionSettings(config.NewComponentIDWithName(typeStr, "all_settings")),
			Directory:         ".",
			Compaction: &CompactionConfig{
				Directory:          ".",
				OnStart:            true,
				MaxTransactionSize: 2048,
			},
			Timeout: 2 * time.Second,
		},
		ext1)
}

func TestHandleNonExistingDirectoryWithAnError(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = "/not/a/dir"

	err := cfg.Validate()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "directory must exist: "))
}

func TestHandleProvidingFilePathAsDirWithAnError(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	file, err := os.CreateTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
		require.NoError(t, os.Remove(file.Name()))
	})

	cfg.Directory = file.Name()

	err = cfg.Validate()
	require.Error(t, err)
	require.EqualError(t, err, file.Name()+" is not a directory")
}
