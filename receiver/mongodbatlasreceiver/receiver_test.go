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
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	recv := receiver{
		cfg: cfg,
	}

	tt := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "initial lookback is now() - collection_interval",
			run: func(t *testing.T) {
				now := time.Now()
				tc := recv.timeConstraints(now)
				require.NotNil(t, tc)
				require.Equal(t, tc.start, now.Add(cfg.CollectionInterval*-1).UTC().Format(time.RFC3339))
				// set lookback for next test after this one is done
				recv.lastRun = now
			},
		},
		{
			name: "lookback for subsequent runs is now() - lastRun",
			run: func(t *testing.T) {
				now := time.Now()
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
