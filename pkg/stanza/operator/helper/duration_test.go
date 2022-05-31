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

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

type durationTestCase struct {
	name        string
	input       string
	expected    Duration
	expectError bool
}

func TestParseDuration(t *testing.T) {
	cases := []durationTestCase{
		{
			"simple nanosecond",
			`"10ns"`,
			Duration{10 * time.Nanosecond},
			false,
		},
		{
			"simple microsecond",
			`"10us"`,
			Duration{10 * time.Microsecond},
			false,
		},
		{
			"simple alternate microsecond",
			`"10µs"`,
			Duration{10 * time.Microsecond},
			false,
		},
		{
			"simple millisecond",
			`"10ms"`,
			Duration{10 * time.Millisecond},
			false,
		},
		{
			"simple second",
			`"1s"`,
			Duration{time.Second},
			false,
		},
		{
			"simple minute",
			`"10m"`,
			Duration{10 * time.Minute},
			false,
		},
		{
			"simple hour",
			`"10h"`,
			Duration{10 * time.Hour},
			false,
		},
		{
			"float hour",
			`"1.5h"`,
			Duration{1.5 * 3600000000000},
			false,
		},
		{
			"number defaults to seconds",
			`10`,
			Duration{10 * time.Second},
			false,
		},
		{
			"float",
			`1.5`,
			Duration{1.5 * 1000000000.0},
			false,
		},
		{
			"int string",
			`"10"`,
			Duration{10 * time.Second},
			false,
		},
		{
			"multi unit",
			`"10h10m10s"`,
			Duration{(10 * time.Second) + (10 * time.Minute) + (10 * time.Hour)},
			false,
		},
		{
			"character",
			`i`,
			Duration{time.Second},
			true,
		},
		{
			"space before unit",
			`"10 s"`,
			Duration{(10 * time.Second)},
			true,
		},
		{
			"capital unit",
			`"10S"`,
			Duration{(10 * time.Second)},
			true,
		},
	}

	for _, tc := range cases {
		t.Run("json/"+tc.name, func(t *testing.T) {
			var dur Duration

			err := json.Unmarshal([]byte(tc.input), &dur)
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, dur)
		})
	}

	additionalCases := []durationTestCase{
		{
			"simple minute unquoted",
			`10m`,
			Duration{10 * time.Minute},
			false,
		},
		{
			"simple second unquoted",
			`10s`,
			Duration{10 * time.Second},
			false,
		},
		{
			"simple multi unquoted",
			`10h10m10s`,
			Duration{(10 * time.Second) + (10 * time.Minute) + (10 * time.Hour)},
			false,
		},
	}

	cases = append(cases, additionalCases...)

	for _, tc := range cases {
		t.Run("yaml/"+tc.name, func(t *testing.T) {
			var dur Duration
			err := yaml.UnmarshalStrict([]byte(tc.input), &dur)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, dur)
		})

		t.Run("mapstructure/"+tc.name, func(t *testing.T) {
			var dur Duration
			var raw string
			_ = yaml.Unmarshal([]byte(tc.input), &raw)

			dc := &mapstructure.DecoderConfig{Result: &dur, DecodeHook: JSONUnmarshalerHook()}
			ms, err := mapstructure.NewDecoder(dc)
			require.NoError(t, err)

			err = ms.Decode(raw)
			if tc.expectError {
				require.Error(t, err)
				return
			}
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
