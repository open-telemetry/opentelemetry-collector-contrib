package aerospikereceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNewAerospikeReceiver_BadEndpoint(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		errMsg   string
	}{
		{
			name:     "no port",
			endpoint: "localhost",
			errMsg:   "missing port in address",
		},
		{
			name:     "no address",
			endpoint: "",
			errMsg:   "missing port in address",
		},
	}

	cs, err := consumer.NewMetrics(func(ctx context.Context, ld pmetric.Metrics) error { return nil })
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{Endpoint: tc.endpoint}
			receiver, err := newAerospikeReceiver(component.ReceiverCreateSettings{}, cfg, cs)
			require.ErrorContains(t, err, tc.errMsg)
			require.Nil(t, receiver)
		})
	}
}
