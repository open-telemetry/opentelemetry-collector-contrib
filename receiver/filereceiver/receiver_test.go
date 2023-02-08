// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereceiver

import (
	"context"
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
	r := &fileReceiver{
		path:   "testdata/metrics.json",
		next:   tc,
		logger: zap.NewNop(),
	}
	err := r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	const numExpectedMetrics = 10
	assert.Eventually(t, func() bool {
		return assert.Equal(t, numExpectedMetrics, len(tc.consumed))
	}, time.Second, 10*time.Millisecond)
	err = r.Shutdown(context.Background())
	assert.NoError(t, err)
}

type testConsumer struct {
	consumed []pmetric.Metrics
}

func (c *testConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (c *testConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	c.consumed = append(c.consumed, md)
	return nil
}
