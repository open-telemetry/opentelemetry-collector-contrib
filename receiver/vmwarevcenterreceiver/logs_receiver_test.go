package vmwarevcenterreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestStart(t *testing.T) {
	f := vcenterReceiverFactory{}
	cases := []struct {
		desc        string
		expectedErr error
		cfg         LoggingConfig
	}{
		{
			desc: "valid start",
		},
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.desc, func(t *testing.T) {
			dc := createDefaultConfig()
			lr, err := f.createLogsReceiver(
				context.Background(),
				componenttest.NewNopReceiverCreateSettings(),
				dc,
				consumertest.NewNop(),
			)
			require.NoError(t, err)

			err = lr.Start(context.Background(), componenttest.NewNopHost())
			if tc.expectedErr != nil {
				require.ErrorIs(t, tc.expectedErr, err)
			}
		})
	}
}

func TestShutdown(t *testing.T) {
	f := vcenterReceiverFactory{}
	dc := createDefaultConfig()
	lr, err := f.createLogsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		dc,
		consumertest.NewNop(),
	)
	require.NoError(t, err)

	err = lr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = lr.Shutdown(context.Background())
	require.NoError(t, err)
}
