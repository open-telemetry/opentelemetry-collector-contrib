package vmwarevcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vmwarevcenterreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateMetricsReceiver(t *testing.T) {
	f := vcenterReceiverFactory{
		receivers: make(map[*Config]*vcenterReceiver),
	}
	testCases := []struct {
		desc   string
		testFn func(t *testing.T)
	}{
		{
			desc: "Default config",
			testFn: func(t *testing.T) {
				t.Parallel()
				_, err := f.createMetricsReceiver(
					context.Background(),
					componenttest.NewNopReceiverCreateSettings(),
					createDefaultConfig(),
					consumertest.NewNop(),
				)
				require.NoError(t, err)
			},
		},
		{
			desc: "Nil config",
			testFn: func(t *testing.T) {
				t.Parallel()

				_, err := f.createMetricsReceiver(
					context.Background(),
					componenttest.NewNopReceiverCreateSettings(),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotVcenter)
			},
		},
		{
			desc: "Nil consumer",
			testFn: func(t *testing.T) {
				t.Parallel()
				_, err := f.createMetricsReceiver(
					context.Background(),
					componenttest.NewNopReceiverCreateSettings(),
					createDefaultConfig(),
					nil,
				)
				require.ErrorIs(t, err, componenterror.ErrNilNextConsumer)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.testFn)
	}
}
