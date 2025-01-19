// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nsxtreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.EqualValues(t, metadata.Type, ft)
}

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	err := component.ValidateConfig(factory.CreateDefaultConfig())
	// default does not endpoint
	require.ErrorContains(t, err, "no manager endpoint was specified")
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(),
		&Config{
			ControllerConfig: scraperhelper.ControllerConfig{
				CollectionInterval: 10 * time.Second,
				InitialDelay:       time.Second,
			},
		},
		consumertest.NewNop(),
	)
	require.NoError(t, err)
}

func TestCreateMetricsNotNSX(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(),
		receivertest.NewNopFactory().CreateDefaultConfig(),
		consumertest.NewNop(),
	)
	require.Error(t, err)
	require.ErrorContains(t, err, errConfigNotNSX.Error())
}
