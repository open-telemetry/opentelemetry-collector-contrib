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

package metadata

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/provider"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils/cache"
)

func TestHost(t *testing.T) {
	// Start with a fresh cache, the following test would fail
	// if the cache key is already set.
	cache.Cache.Delete(cache.CanonicalHostnameKey)

	p, err := buildCurrentProvider(componenttest.NewNopTelemetrySettings(), "test-host")
	require.NoError(t, err)
	src, err := p.Source(context.Background())
	require.NoError(t, err)
	assert.Equal(t, src.Identifier, "test-host")

	// config.Config.Hostname does not get stored in the cache
	p, err = buildCurrentProvider(componenttest.NewNopTelemetrySettings(), "test-host-2")
	require.NoError(t, err)
	src, err = p.Source(context.Background())
	require.NoError(t, err)
	assert.Equal(t, src.Identifier, "test-host-2")

	// Disable EC2 Metadata service to prevent fetching hostname from there,
	// in case the test is running on an EC2 instance
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	p, err = buildCurrentProvider(componenttest.NewNopTelemetrySettings(), "")
	require.NoError(t, err)
	src, err = p.Source(context.Background())
	require.NoError(t, err)
	osHostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Contains(t, src.Identifier, osHostname)
}

var _ source.Provider = (*ErrorSourceProvider)(nil)

type ErrorSourceProvider string

func (p ErrorSourceProvider) Source(context.Context) (source.Source, error) {
	return source.Source{}, errors.New(string(p))
}

func TestWarnProvider(t *testing.T) {
	tests := []struct {
		name            string
		curProvider     source.Provider
		previewProvider source.Provider

		expectedLogs []observer.LoggedEntry
		src          source.Source
		err          string
	}{
		{
			name:            "current provider fails",
			curProvider:     ErrorSourceProvider("errorCurrentHostname"),
			previewProvider: provider.Config("previewHostname"),
			err:             "errorCurrentHostname",
		},
		{
			name:            "preview provider fails",
			curProvider:     provider.Config("currentHostname"),
			previewProvider: ErrorSourceProvider("errorPreviewHostname"),
			src:             source.Source{Kind: source.HostnameKind, Identifier: "currentHostname"},
			expectedLogs: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.WarnLevel,
						Message: previewHostnameFailedLogMessage,
					},
					Context: []zapcore.Field{
						{
							Key:       "error",
							Type:      zapcore.ErrorType,
							Interface: errors.New("errorPreviewHostname"),
						},
					},
				},
			},
		},
		{
			name:            "preview provider and current provider match",
			curProvider:     provider.Config("hostname"),
			previewProvider: provider.Config("hostname"),
			src:             source.Source{Kind: source.HostnameKind, Identifier: "hostname"},
		},
		{
			name:            "preview provider and current provider don't match",
			curProvider:     provider.Config("currentHostname"),
			previewProvider: provider.Config("previewHostname"),
			src:             source.Source{Kind: source.HostnameKind, Identifier: "currentHostname"},
			expectedLogs: []observer.LoggedEntry{
				{
					Entry: zapcore.Entry{
						Level:   zap.WarnLevel,
						Message: defaultHostnameChangeLogMessage,
					},
					Context: []zapcore.Field{
						{
							Key:       "current default source",
							Type:      zapcore.ReflectType,
							Interface: source.Source{Kind: source.HostnameKind, Identifier: "currentHostname"},
						},
						{
							Key:       "future default source",
							Type:      zapcore.ReflectType,
							Interface: source.Source{Kind: source.HostnameKind, Identifier: "previewHostname"},
						},
					},
				},
			},
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			core, observed := observer.New(zapcore.DebugLevel)
			provider := &warnProvider{
				logger:          zap.New(core),
				curProvider:     testInstance.curProvider,
				previewProvider: testInstance.previewProvider,
			}

			src, err := provider.Source(context.Background())
			if err != nil || testInstance.err != "" {
				assert.EqualError(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.src, src)
			}
			assert.ElementsMatch(t,
				testInstance.expectedLogs,
				observed.AllUntimed(),
			)
		})
	}
}
