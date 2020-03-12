package redisreceiver

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/require"
)

func TestIntervalRunner(t *testing.T) {
	runner := newRedisTickable(context.Background(), newFakeClient(), &fakeMetricsConsumer{})
	err := runner.setup()
	require.Nil(t, err)
	err = runner.run()
	require.Nil(t, err)
}

type fakeMetricsConsumer struct{}

func (c fakeMetricsConsumer) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	return nil
}
