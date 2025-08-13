// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expvarreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.Equal(t, metadata.Type, ft)
}

func TestValidConfig(t *testing.T) {
	factory := NewFactory()
	err := componenttest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	metricsReceiver, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		&Config{
			ControllerConfig: scraperhelper.ControllerConfig{
				CollectionInterval: 10 * time.Second,
			},
			ClientConfig:         clientConfig,
			MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		},
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)
}
