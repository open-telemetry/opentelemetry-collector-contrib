// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apachereceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		endpoint    string
		errExpected bool
		errText     string
	}{
		{
			desc:        "default_endpoint",
			endpoint:    "http://localhost:8080/server-status?auto",
			errExpected: false,
		},
		{
			desc:        "custom_host",
			endpoint:    "http://123.123.123.123:8080/server-status?auto",
			errExpected: false,
		},
		{
			desc:        "custom_port",
			endpoint:    "http://123.123.123.123:9090/server-status?auto",
			errExpected: false,
		},
		{
			desc:        "custom_path",
			endpoint:    "http://localhost:8080/my-status?auto",
			errExpected: false,
		},
		{
			desc:        "empty_path",
			endpoint:    "",
			errExpected: true,
			errText:     "missing hostname: ''",
		},
		{
			desc:        "missing_hostname",
			endpoint:    "http://:8080/server-status?auto",
			errExpected: true,
			errText:     "missing hostname: 'http://:8080/server-status?auto'",
		},
		{
			desc:        "missing_query",
			endpoint:    "http://localhost:8080/server-status",
			errExpected: true,
			errText:     "query must be 'auto': 'http://localhost:8080/server-status'",
		},
		{
			desc:        "invalid_query",
			endpoint:    "http://localhost:8080/server-status?nonsense",
			errExpected: true,
			errText:     "query must be 'auto': 'http://localhost:8080/server-status?nonsense'",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.Endpoint = tc.endpoint
			err := component.ValidateConfig(cfg)
			if tc.errExpected {
				require.EqualError(t, err, tc.errText)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(typeStr, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://localhost:8080/server-status?auto"
	expected.CollectionInterval = 10 * time.Second

	require.Equal(t, expected, cfg)
}
