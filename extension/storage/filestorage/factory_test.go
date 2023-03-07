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
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, typeStr, f.Type())

	cfg := f.CreateDefaultConfig().(*Config)

	if runtime.GOOS != "windows" {
		require.Equal(t, "/var/lib/otelcol/file_storage", cfg.Directory)
	} else {
		expected := filepath.Join(os.Getenv("ProgramData"), "Otelcol", "FileStorage")
		require.Equal(t, expected, cfg.Directory)
	}
	require.Equal(t, time.Second, cfg.Timeout)

	tests := []struct {
		name           string
		config         *Config
		wantErr        bool
		wantErrMessage string
	}{
		{
			name: "Default",
			config: func() *Config {
				return &Config{
					Directory:  t.TempDir(),
					Compaction: &CompactionConfig{},
				}
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, err := f.CreateExtension(
				context.Background(),
				extensiontest.NewNopCreateSettings(),
				test.config,
			)
			if test.wantErr {
				require.Error(t, err)
				if test.wantErrMessage != "" {
					require.True(t, strings.HasPrefix(err.Error(), test.wantErrMessage))
				}
				require.Nil(t, e)
			} else {
				require.NoError(t, err)
				require.NotNil(t, e)
				ctx := context.Background()
				require.NoError(t, e.Start(ctx, componenttest.NewNopHost()))
				require.NoError(t, e.Shutdown(ctx))
			}
		})
	}
}
