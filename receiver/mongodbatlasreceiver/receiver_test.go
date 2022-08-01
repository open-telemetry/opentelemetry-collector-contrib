// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbatlasreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := componenttest.NewNopReceiverCreateSettings()
	ctx := context.Background()

	receiver, err := createMetricsReceiver(
		ctx,
		params,
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, receiver, "receiver creation failed")

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

func TestTimeConstraints(t *testing.T) {
	tt := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "initial lookback is now() - collection_interval",
			run: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				// lastRun is nil
				recv := receiver{
					cfg: cfg,
				}
				now := time.Now()
				tc := recv.timeConstraints(now)
				require.NotNil(t, tc)
				require.Equal(t, tc.start, now.Add(cfg.CollectionInterval*-1).UTC().Format(time.RFC3339))
			},
		},
		{
			name: "lookback for subsequent runs is now() - lastRun",
			run: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				now := time.Now()
				recv := receiver{
					cfg: cfg,
					// set last run to 1 collection ago
					lastRun: now.Add(cfg.CollectionInterval * -1),
				}
				tc := recv.timeConstraints(now)
				require.NotNil(t, tc)
				require.Equal(t, tc.start, recv.lastRun.UTC().Format(time.RFC3339))
			},
		},
	}

	for _, testCase := range tt {
		t.Run(testCase.name, testCase.run)
	}
}
