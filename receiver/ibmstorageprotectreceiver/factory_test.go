package ibmstorageprotectreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{"testdata/metrics.json"}
	cfg.Config.StartAt = "beginning"
	metricsSink := new(consumertest.MetricsSink)

	receiver, err := factory.CreateMetricsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		cfg,
		metricsSink,
	)
	require.NoError(t, err)
	assert.NotNil(t, receiver)
}
