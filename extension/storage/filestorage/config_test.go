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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(typeStr),
			expected: func() component.Config {
				ret := NewFactory().CreateDefaultConfig()
				ret.(*Config).Directory = "."
				return ret
			}(),
		},
		{
			id: component.NewIDWithName(typeStr, "all_settings"),
			expected: &Config{
				Directory: ".",
				Compaction: &CompactionConfig{
					Directory:                  ".",
					OnStart:                    true,
					OnRebound:                  true,
					MaxTransactionSize:         2048,
					ReboundTriggerThresholdMiB: 16,
					ReboundNeededThresholdMiB:  128,
					CheckInterval:              time.Second * 5,
				},
				Timeout: 2 * time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestHandleNonExistingDirectoryWithAnError(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = "/not/a/dir"

	err := component.ValidateConfig(cfg)
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

	err = component.ValidateConfig(cfg)
	require.Error(t, err)
	require.EqualError(t, err, file.Name()+" is not a directory")
}
