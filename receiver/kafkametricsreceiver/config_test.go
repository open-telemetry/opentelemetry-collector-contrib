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

package kafkametricsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(typeStr, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, &Config{
		ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(typeStr),
		Brokers:                   []string{"10.10.10.10:9092"},
		ProtocolVersion:           "2.0.0",
		TopicMatch:                "test_\\w+",
		GroupMatch:                "test_\\w+",
		Authentication: kafkaexporter.Authentication{
			TLS: &configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile:   "ca.pem",
					CertFile: "cert.pem",
					KeyFile:  "key.pem",
				},
			},
		},
		ClientID:             defaultClientID,
		Scrapers:             []string{"brokers", "topics", "consumers"},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}, cfg)
}
