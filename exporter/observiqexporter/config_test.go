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

package observiqexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		testName    string
		input       Config
		shouldError bool
	}{
		{
			testName: "Valid config passes",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: defaultEndpoint,
			},
			shouldError: false,
		},
		{
			testName: "Empty APIKey fails",
			input: Config{
				APIKey:   "",
				Endpoint: defaultEndpoint,
			},
			shouldError: true,
		},
		{
			testName: "Empty Endpoint fails",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: "",
			},
			shouldError: true,
		},
		{
			testName: "Non-url endpoint fails",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: "cache_object:foo/bar",
			},
			shouldError: true,
		},
		{
			testName: "Non http or https url fails",
			input: Config{
				APIKey:   "11111111-2222-3333-4444-555555555555",
				Endpoint: "ftp://app.com",
			},
			shouldError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			err := testCase.input.validateConfig()
			if testCase.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
