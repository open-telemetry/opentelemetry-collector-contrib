// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
