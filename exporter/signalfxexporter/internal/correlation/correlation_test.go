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

package correlation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestTrackerAddSpans(t *testing.T) {
	tracker := NewTracker(
		DefaultConfig(),
		"abcd",
		componenttest.NewNopExporterCreateSettings(),
	)

	err := tracker.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, tracker.correlation, "correlation context should be set")

	traces := pdata.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	attr := rs.Resource().Attributes()
	attr.InsertString("host.name", "localhost")

	// Add empty first, should ignore.
	tracker.AddSpans(context.Background(), pdata.NewTraces())
	assert.Nil(t, tracker.traceTracker)

	tracker.AddSpans(context.Background(), traces)

	assert.NotNil(t, tracker.traceTracker, "trace tracker should be set")

	assert.NoError(t, tracker.Shutdown(context.Background()))
}

func TestTrackerStart(t *testing.T) {

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "invalid http client settings fails",
			config: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "localhost:9090",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "/non/existent",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "failed to create correlation API client: failed to load TLS config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker(
				tt.config,
				"abcd",
				componenttest.NewNopExporterCreateSettings(),
			)

			err := tracker.Start(context.Background(), componenttest.NewNopHost())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
