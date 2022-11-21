// Copyright  The OpenTelemetry Authors
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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestConfigValidation(t *testing.T) {
	testcases := []struct {
		name         string
		cfg          *Config
		wantErr      bool
		errorMessage string
	}{
		{
			name: "empty endpoint",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "",
				},
				LogBatchingInterval: 200 * time.Millisecond,
			},
			wantErr:      true,
			errorMessage: "Endpoint should not be empty",
		},
		{
			name: "missing http scheme",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "test.com/dummy",
				},
				LogBatchingInterval: 200 * time.Millisecond,
			},
			wantErr:      true,
			errorMessage: "Endpoint must be valid",
		},
		{
			name: "invalid endpoint format",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "invalid.com@#$%",
				},
				LogBatchingInterval: 200 * time.Millisecond,
			},
			wantErr:      true,
			errorMessage: "Endpoint must be valid",
		},
		{
			name: "minimum batching interval",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://validurl.com/rest",
				},
				LogBatchingInterval: 20 * time.Millisecond,
			},
			wantErr:      true,
			errorMessage: "Minimum log batching interval should be 30ms",
		},
		{
			name: "valid config",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://validurl.com/rest",
				},
				LogBatchingInterval: 200 * time.Millisecond,
			},
			wantErr:      false,
			errorMessage: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("config validation failed: error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if tc.wantErr {
				assert.Error(t, err)
				if len(tc.errorMessage) != 0 {
					assert.Equal(t, errors.New(tc.errorMessage), err, "Error messages must match")
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}
