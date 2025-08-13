// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.Equal(t, metadata.Type, ft)
}

func TestValidConfig(t *testing.T) {
	factory := NewFactory()
	require.NoError(t, xconfmap.Validate(factory.CreateDefaultConfig()))
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
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
