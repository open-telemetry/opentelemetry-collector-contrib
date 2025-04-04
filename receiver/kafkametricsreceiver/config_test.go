// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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

	expectedClientConfig := configkafka.NewDefaultClientConfig()
	expectedClientConfig.Brokers = []string{"10.10.10.10:9092"}
	expectedClientConfig.Metadata.Full = false
	expectedClientConfig.Metadata.RefreshInterval = time.Nanosecond // set by refresh_frequency

	assert.Equal(t, &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		ClientConfig:     expectedClientConfig,

		ClusterAlias:         "kafka-test",
		TopicMatch:           "test_\\w+",
		GroupMatch:           "test_\\w+",
		RefreshFrequency:     time.Nanosecond,
		Scrapers:             []string{"brokers", "topics", "consumers"},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}, cfg)
}
