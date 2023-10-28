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

func TestCreateTracesReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Trace.Encoding = "unknown"
	cfg.Trace.Topic = "pulsar://public/default/otlp_metrics"

	r, err := createTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateTracesReceiver_err_topic(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Trace.Encoding = defaultEncoding
	cfg.Trace.Topic = defaultTopicName

	r, err := createTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateTraceReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Trace.Encoding = defaultEncoding
	cfg.Trace.Topic = "pulsar://public/default/otlp_spans"

	recv, err := createTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}
