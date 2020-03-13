package redisreceiver

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/require"
)

func TestIntervalRunner(t *testing.T) {
	consumer := &fakeMetricsConsumer{}
	runner := newRedisRunnable(context.Background(), newFakeClient(), consumer, nil)
	err := runner.setup()
	require.Nil(t, err)
	err = runner.run()
	require.Nil(t, err)
	require.Equal(t, 29, len(consumer.md.Metrics))
}

type fakeMetricsConsumer struct{
	md consumerdata.MetricsData
}

func (c *fakeMetricsConsumer) ConsumeMetricsData(ctx context.Context, md consumerdata.MetricsData) error {
	c.md = md
	return nil
}
