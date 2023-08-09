// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chronyreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	assert.Equal(t, component.Type("chrony"), factory.Type(), "Must match the expected type")
}

func TestValidConfig(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	assert.NoError(t, componenttest.CheckConfigStruct(factory.CreateDefaultConfig()))
}

func TestCreatingMetricsReceiver(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics = metadata.MetricsConfig{
		NtpTimeCorrection: metadata.MetricConfig{
			Enabled: true,
		},
		NtpSkew: metadata.MetricConfig{
			Enabled: true,
		},
	}
	mem, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		&Config{
			ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
				CollectionInterval: 30 * time.Second,
			},
			MetricsBuilderConfig: mbc,
			Endpoint:             "udp://localhost:323",
			Timeout:              10 * time.Second,
		},
		consumertest.NewNop(),
	)
	assert.NoError(t, err, "Must not error creating metrics receiver")
	assert.NotNil(t, mem, "Must have a valid metrics receiver client")
}
