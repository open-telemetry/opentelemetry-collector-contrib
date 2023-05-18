// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filereceiver

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestReceiver(t *testing.T) {
	tc := &testConsumer{}
	cons := consumerType{
		metricsConsumer: tc,
	}
	r := &fileReceiver{
		path:     "testdata/metrics.json",
		consumer: cons,
		logger:   zap.NewNop(),
	}
	err := r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.Eventually(t, func() bool {
		const numExpectedMetrics = 10
		return numExpectedMetrics == tc.numConsumed()
	}, 2*time.Second, 100*time.Millisecond)
	err = r.Shutdown(context.Background())
	assert.NoError(t, err)
}

type testConsumer struct {
	consumed []pmetric.Metrics
	mu       sync.Mutex
}

func (c *testConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *testConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consumed = append(c.consumed, md)
	return nil
}

func (c *testConsumer) numConsumed() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.consumed)
}
