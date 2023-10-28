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

func TestCreateLogsReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Log.Encoding = "unknown"
	cfg.Log.Topic = "pulsar://public/default/otlp_logs"
	r, err := createLogsReceiver(context.TODO(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateLogsReceiver_err_topic(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Log.Encoding = defaultEncoding
	cfg.Log.Topic = defaultTopicName
	r, err := createLogsReceiver(context.TODO(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL
	cfg.Log.Encoding = defaultEncoding
	cfg.Log.Topic = "pulsar://public/default/otlp_logs"

	recv, err := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}
