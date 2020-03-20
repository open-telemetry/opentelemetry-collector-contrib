package redisreceiver

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRedisRunnable(t *testing.T) {
	consumer := &fakeMetricsConsumer{}
	logger, _ := zap.NewDevelopment()
	runner := newRedisRunnable(context.Background(), newFakeClient(), consumer, logger)
	err := runner.Setup()
	require.Nil(t, err)
	err = runner.Run()
	require.Nil(t, err)
	// + 6 because there are two keyspace entries each of which has three metrics
	require.Equal(t, len(getDefaultRedisMetrics())+6, len(consumer.md.Metrics))
}

type fakeMetricsConsumer struct {
	md consumerdata.MetricsData
}

func (c *fakeMetricsConsumer) ConsumeMetricsData(_ context.Context, md consumerdata.MetricsData) error {
	c.md = md
	return nil
}
