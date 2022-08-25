// Copyright The OpenTelemetry Authors
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

package logicmonitorexporter

import (
	"testing"
	"time"

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
			name: "empty URL",
			cfg: &Config{
				URL: "",
			},
			expectedErr: "URL should not be empty",
		},
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
			name: "valid config",
			cfg: &Config{
				URL:                 "http://validurl.com/rest",
				LogBatchingInterval: 200 * time.Millisecond,
			},
		},
		{
			name: "minimum batching interval",
			cfg: &Config{
				URL:                 "http://validurl.com/rest",
				LogBatchingInterval: 20 * time.Millisecond,
			},
			expectedErr: "Minimum log batching interval should be 30ms",
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
