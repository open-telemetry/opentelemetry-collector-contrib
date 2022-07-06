// Copyright  The OpenTelemetry Authors
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

package chronyreceiver

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err, "Must not error on creating Nop factories")

	factory := NewFactory()
	factories.Receivers[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(path.Join("testdata", "config.yml"), factories)
	require.NoError(t, err, "Must not error when loading configuration")

	chronyConf := cfg.Receivers[config.NewComponentIDWithName(typeStr, "custom")]
	expect := &Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(typeStr),
		MetricsSettings:           metadata.DefaultMetricsSettings(),
		Endpoint:                  "udp://localhost:3030",
		Timeout:                   10 * time.Second,
	}
	expect.SetIDName("custom")
	assert.Equal(t, expect, chronyConf)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		conf     Config
		err      error
	}{
		{
			scenario: "Valid udp configuration",
			conf: Config{
				Endpoint: "udp://localhost:323",
				Timeout:  10 * time.Second,
			},
			err: nil,
		},
		{
			scenario: "Invalid udp hostname",
			conf: Config{
				Endpoint: "udp://:323",
				Timeout:  10 * time.Second,
			},
			err: chrony.ErrInvalidNetwork,
		},
		{
			scenario: "Invalid udp port",
			conf: Config{
				Endpoint: "udp://localhost",
				Timeout:  10 * time.Second,
			},
			err: chrony.ErrInvalidNetwork,
		},
		{
			scenario: "Valid unix path",
			conf: Config{
				Endpoint: fmt.Sprintf("unix://%s", t.TempDir()),
				Timeout:  10 * time.Second,
			},
			err: nil,
		},
		{
			scenario: "Invalid unix path",
			conf: Config{
				Endpoint: "unix:///no/dir/to/socket",
				Timeout:  10 * time.Second,
			},
			err: os.ErrNotExist,
		},
		{
			scenario: "Invalid timeout set",
			conf: Config{
				Endpoint: "unix://no/dir/to/socket",
				Timeout:  0,
			},
			err: errInvalidValue,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			assert.ErrorIs(t, tc.conf.Validate(), tc.err, "Must match the expected error")
		})
	}
}
