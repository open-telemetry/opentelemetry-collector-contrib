// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t, &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		ClusterAlias:     "kafka-test",
		Brokers:          []string{"10.10.10.10:9092"},
		ProtocolVersion:  "2.0.0",
		TopicMatch:       "test_\\w+",
		GroupMatch:       "test_\\w+",
		Authentication: configkafka.AuthenticationConfig{
			TLS: &configtls.ClientConfig{
				Config: configtls.Config{
					CAFile:   "ca.pem",
					CertFile: "cert.pem",
					KeyFile:  "key.pem",
				},
			},
		},
		RefreshFrequency:     1,
		ClientID:             defaultClientID,
		Scrapers:             []string{"brokers", "topics", "consumers"},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}, cfg)
}
