// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected Duration
	}{
		{
			"simple second",
			`"1s"`,
			Duration{time.Second},
		},
		{
			"simple minute",
			`"10m"`,
			Duration{10 * time.Minute},
		},
		{
			"number defaults to seconds",
			`10`,
			Duration{10 * time.Second},
		},
	}

	for _, tc := range cases {
		t.Run("yaml "+tc.name, func(t *testing.T) {
			var dur Duration
			err := yaml.UnmarshalStrict([]byte(tc.input), &dur)
			require.NoError(t, err)
			require.Equal(t, tc.expected, dur)
		})

		t.Run("json "+tc.name, func(t *testing.T) {
			var dur Duration
			err := json.Unmarshal([]byte(tc.input), &dur)
			require.NoError(t, err)
			require.Equal(t, tc.expected, dur)
		})
	}
}

func TestParseDurationRoundtrip(t *testing.T) {
	cases := []struct {
		name  string
		input Duration
	}{
		{
			"zero",
			Duration{},
		},
		{
			"second",
			Duration{time.Second},
		},
		{
			"minute",
			Duration{10 * time.Minute},
		},
	}

	for _, tc := range cases {
		t.Run("yaml "+tc.name, func(t *testing.T) {
			durBytes, err := yaml.Marshal(tc.input)
			require.NoError(t, err)

			var dur Duration
			err = yaml.UnmarshalStrict(durBytes, &dur)
			require.NoError(t, err)
			require.Equal(t, tc.input, dur)
		})

		t.Run("json "+tc.name, func(t *testing.T) {
			durBytes, err := json.Marshal(tc.input)
			require.NoError(t, err)

			var dur Duration
			err = json.Unmarshal(durBytes, &dur)
			require.NoError(t, err)
			require.Equal(t, tc.input, dur)
		})
	}
}
