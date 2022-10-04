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

package awscloudwatchreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name: "Valid Config",
			config: Config{
				Region: "us-west-2",
				Logs: LogsConfig{
					EventLimit:   defaultEventLimit,
					PollInterval: defaultPollInterval,
				},
			},
		},
		{
			name: "Invalid No Region",
			config: Config{
				Region: "",
			},
			expectedErr: errNoRegion,
		},
		{
			name: "Invalid Event Limit",
			config: Config{
				Region: "us-west-2",
				Logs: LogsConfig{
					EventLimit: -1,
				},
			},
			expectedErr: errInvalidEventLimit,
		},
		{
			name: "Invalid Poll Interval",
			config: Config{
				Region: "us-west-2",
				Logs: LogsConfig{
					EventLimit:   defaultEventLimit,
					PollInterval: 100 * time.Millisecond,
				},
			},
			expectedErr: errInvalidPollInterval,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
