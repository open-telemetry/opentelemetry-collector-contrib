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

package ptracetest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
)

func TestCompareTraces(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareTracesOption
		withoutOptions internal.Expectation
		withOptions    internal.Expectation
	}{
		{
			name: "equal",
		},
		{
			name: "ignore-one-resource-attribute",
			compareOptions: []CompareTracesOption{
				IgnoreResourceAttributeValue("host.name"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[host.name:different-node1]"),
					errors.New("extra resource with attributes: map[host.name:host1]"),
				),
				Reason: "An unpredictable resource attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "The unpredictable resource attribute was ignored on each resource that carried it.",
			},
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareTracesOption{
				IgnoreResourceSpansOrder(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceTraces with attributes map[host.name:host1] expected at index 0, found a at index 1"),
					errors.New("ResourceTraces with attributes map[host.name:host2] expected at index 1, found a at index 0"),
				),
				Reason: "Resource order mismatch will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "Ignored resource order mismatch should not cause a failure.",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)

			expected, err := golden.ReadTraces(filepath.Join(dir, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadTraces(filepath.Join(dir, "actual.json"))
			require.NoError(t, err)

			err = CompareTraces(expected, actual)
			tc.withoutOptions.Validate(t, err)

			if tc.compareOptions == nil {
				return
			}

			err = CompareTraces(expected, actual, tc.compareOptions...)
			tc.withOptions.Validate(t, err)
		})
	}
}
