// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestDefaultFactory(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Region = "eu-west-1"

	sink := &consumertest.MetricsSink{}
	mtrcRcvr := newMetricReceiver(cfg, zap.NewNop(), sink)

	err := mtrcRcvr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = mtrcRcvr.Shutdown(context.Background())
	require.NoError(t, err)
}
