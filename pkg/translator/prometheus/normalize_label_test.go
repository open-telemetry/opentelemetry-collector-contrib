// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestNormalizeLabel(t *testing.T) {
	testCases := []struct {
		label           string
		utfAllowed      bool
		dropSantization bool
		expected        string
	}{
		{
			label:    "",
			expected: "",
		},
		{
			label:    "test",
			expected: "test",
		},
		{
			label:    "__test",
			expected: "__test",
		},
		{
			label:    "_test",
			expected: "key_test",
		},
		{
			label:           "_test",
			dropSantization: true,
			expected:        "_test",
		},
		{
			label:    "0test",
			expected: "key_0test",
		},
		{
			label:           "0test",
			dropSantization: true,
			expected:        "key_0test",
		},
		{
			label:    "test_/",
			expected: "test__",
		},
		{
			label:      "test_/",
			utfAllowed: true,
			expected:   "test_/",
		},
		{
			label:    "test.test",
			expected: "test_test",
		},
		{
			label:      "test.test",
			utfAllowed: true,
			expected:   "test.test",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			testutil.SetFeatureGateForTest(t, dropSanitizationGate, tc.dropSantization)
			testutil.SetFeatureGateForTest(t, AllowUTF8FeatureGate, tc.utfAllowed)
			require.Equal(t, tc.expected, NormalizeLabel(tc.label))
		})
	}
}
