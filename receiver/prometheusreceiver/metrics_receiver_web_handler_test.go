package prometheusreceiver

import (
	"context"
	"net"
	"testing"

	"github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestInitWebHandler(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	config := &Config{
		PrometheusConfig:         cfg,
		EnablePrometheusUIServer: true,
	}
	cms := new(consumertest.MetricsSink)
	receiver := newPrometheusReceiver(receivertest.NewNopCreateSettings(), config, cms)
	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
	// Add assertions here to verify the behavior of the initPrometheusComponents function
	require.NotNil(t, receiver.webHandler)

	conn, err := net.Dial("tcp", "localhost:9090")
	if err != nil {
		t.Errorf("Failed to connect to port 9090: %v", err)
	}
	conn.Close()
	
	require.NoError(t, receiver.Shutdown(context.Background()))
}

func TestNoWebHandler(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	config := &Config{
		PrometheusConfig:         cfg,
		StartTimeMetricRegex:     "",
		EnablePrometheusUIServer: false,
	}
	cms := new(consumertest.MetricsSink)
	receiver := newPrometheusReceiver(receivertest.NewNopCreateSettings(), config, cms)
	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
	// Add assertions here to verify the behavior of the initPrometheusComponents function
	require.Nil(t, receiver.webHandler)

	_, err := net.Dial("tcp", "localhost:9090")
	require.Error(t, err)
	
	require.NoError(t, receiver.Shutdown(context.Background()))
}
