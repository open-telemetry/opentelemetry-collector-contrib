package lmexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidation(t *testing.T) {
	testcases := []struct {
		name        string
		cfg         *Config
		expectedErr string
	}{
		{
			name: "missing http scheme",
			cfg: &Config{
				URL: "test.com/dummy",
			},
			expectedErr: "URL must be valid",
		},
		{
			name: "invalid url format",
			cfg: &Config{
				URL: "#$#%54345fdsrerw",
			},
			expectedErr: "URL must be valid",
		},
		{
			name: "valid url",
			cfg: &Config{
				URL: "http://validurl.com/rest",
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			err := tc.cfg.Validate()

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				require.NotNil(t, err)
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
