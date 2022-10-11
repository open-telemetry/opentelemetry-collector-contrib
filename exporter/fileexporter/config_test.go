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

package fileexporter

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           config.ComponentID
		expected     config.Exporter
		errorMessage string
	}{
		{
			id: config.NewComponentIDWithName(typeStr, "2"),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				Path:             "./filename.json",
				Rotation: &Rotation{
					MaxMegabytes: 10,
					MaxDays:      3,
					MaxBackups:   3,
					LocalTime:    true,
				},
				FormatType: formatTypeJSON,
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "3"),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				Path:             "./filename",
				Rotation: &Rotation{
					MaxMegabytes: 10,
					MaxDays:      3,
					MaxBackups:   3,
					LocalTime:    true,
				},
				FormatType:  formatTypeProto,
				Compression: compressionZSTD,
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "rotation_with_default_settings"),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				Path:             "./foo",
				FormatType:       formatTypeJSON,
				Rotation: &Rotation{
					MaxBackups: defaultMaxBackups,
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "rotation_with_custom_settings"),
			expected: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
				Path:             "./foo",
				Rotation: &Rotation{
					MaxMegabytes: 1234,
					MaxBackups:   defaultMaxBackups,
				},
				FormatType: formatTypeJSON,
			},
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "compression_error"),
			errorMessage: "compression is not supported",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "format_error"),
			errorMessage: "format type is not supported",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, ""),
			errorMessage: "path must be non-empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalExporter(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, cfg.Validate(), tt.errorMessage)
				return
			}

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
