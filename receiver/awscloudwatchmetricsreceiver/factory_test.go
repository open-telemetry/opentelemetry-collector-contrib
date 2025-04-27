// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver/internal/metadata"
)

func TestFactoryType(t *testing.T) {
	factory := NewFactory()
	ft := factory.Type()
	require.Equal(t, metadata.Type, ft)
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "eu-west-2"
	_, err := NewFactory().CreateMetrics(
		context.Background(), receivertest.NewNopSettings(metadata.Type),
		cfg, nil)
	require.NoError(t, err)
}
