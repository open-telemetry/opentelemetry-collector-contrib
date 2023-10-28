// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"
import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateMetricsReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Metric.Encoding = "unknown"
	cfg.Metric.Topic = "pulsar://public/default/otlp_metrics"

	r, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsReceiver_err_topic(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Metric.Encoding = "unknown"
	cfg.Metric.Topic = defaultTopicName

	r, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Metric.Encoding = defaultEncoding
	cfg.Metric.Topic = "pulsar://public/default/otlp_metrics"

	recv, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}
