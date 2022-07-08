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

package redisreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

func TestConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)
	factory := NewFactory()
	factories.Receivers[factory.Type()] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t,
		&Config{
			NetAddr: confignet.NetAddr{
				Endpoint:  "localhost:6379",
				Transport: "tcp",
			},
			TLS: configtls.TLSClientSetting{
				Insecure: true,
			},
			Password: "test",
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
				CollectionInterval: 10 * time.Second,
			},
			Metrics: metadata.DefaultMetricsSettings(),
		},
		cfg.Receivers[config.NewComponentID(typeStr)],
	)
}
