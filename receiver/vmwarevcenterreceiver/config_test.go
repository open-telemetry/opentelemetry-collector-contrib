package vmwarevcenterreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		desc        string
		cfg         Config
		expectedErr error
	}{
		{
			desc: "empty endpoint",
			cfg: Config{
				MetricsConfig: &MetricsConfig{
					Endpoint: "",
				},
			},
		},
	}

	for _, tc := range cases {
		err := tc.cfg.Validate()
		if tc.expectedErr != nil {
			require.ErrorIs(t, err, tc.expectedErr)
		}
	}

}
