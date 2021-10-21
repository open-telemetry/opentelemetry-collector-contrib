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

package httpforwarder

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	expectType := "http_forwarder"
	require.Equal(t, config.Type(expectType), f.Type())

	cfg := f.CreateDefaultConfig().(*Config)
	require.Equal(t, config.NewComponentID(typeStr), cfg.ID())
	require.Equal(t, ":6060", cfg.Ingress.Endpoint)
	require.Equal(t, 10*time.Second, cfg.Egress.Timeout)

	tests := []struct {
		name           string
		config         *Config
		wantErr        bool
		wantErrMessage string
	}{
		{
			name:           "Default config",
			config:         cfg,
			wantErr:        true,
			wantErrMessage: "'egress.endpoint' config option cannot be empty",
		},
		{
			name:           "Invalid config",
			config:         &Config{Egress: confighttp.HTTPClientSettings{Endpoint: "123.456.7.89:9090"}},
			wantErr:        true,
			wantErrMessage: "enter a valid URL for 'egress.endpoint': parse \"123.456.7.89:9090\": first path segment in URL cannot",
		},
		{
			name:   "Valid config",
			config: &Config{Egress: confighttp.HTTPClientSettings{Endpoint: "localhost:9090"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, err := f.CreateExtension(
				context.Background(),
				componenttest.NewNopExtensionCreateSettings(),
				test.config,
			)
			if test.wantErr {
				if test.wantErrMessage != "" {
					require.True(t, strings.Contains(err.Error(), test.wantErrMessage))
				}
				require.Error(t, err)
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
