// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestSanitize(t *testing.T) {
	oldSanitization := dropSanitizationGate.IsEnabled()
	defer func() {
		testutil.SetFeatureGateForTest(t, dropSanitizationGate, oldSanitization)
	}()

	for _, dropSanitization := range []bool{true, false} {
		testutil.SetFeatureGateForTest(t, dropSanitizationGate, dropSanitization)

		// Drop sanitization gate is only relevant if UTF8 isn't allowed. We use `false` for all cases.
		if dropSanitization {
			require.Equal(t, "", NormalizeLabel("", false), "")
			require.Equal(t, "_test", NormalizeLabel("_test", false))
			require.Equal(t, "key_0test", NormalizeLabel("0test", false)) // Even dropping santiization, we still add "key_" to metrics starting with digits
			require.Equal(t, "test", NormalizeLabel("test", false))
			require.Equal(t, "test__", NormalizeLabel("test_/", false))
			require.Equal(t, "__test", NormalizeLabel("__test", false))
		} else {
			require.Equal(t, "", NormalizeLabel("", false), "")
			require.Equal(t, "key_test", NormalizeLabel("_test", false))
			require.Equal(t, "key_0test", NormalizeLabel("0test", false))
			require.Equal(t, "test", NormalizeLabel("test", false))
			require.Equal(t, "test__", NormalizeLabel("test_/", false))
			require.Equal(t, "__test", NormalizeLabel("__test", false))
		}
	}

}

func TestNormalizeLabel(t *testing.T) {
	tests := []struct {
		label     string
		allowUTF8 bool
		expected  string
	}{
		{"", false, ""},
		{"", true, ""},
		{"label_with_special_chars!", false, "label_with_special_chars_"},
		{"label_with_special_chars!", true, "label_with_special_chars!"},
		{"label_with_foreign_characteres_字符", false, "label_with_foreign_characteres___"},
		{"label_with_foreign_characteres_字符", true, "label_with_foreign_characteres_字符"},
		{"label.with.dots", false, "label_with_dots"},
		{"label.with.dots", true, "label.with.dots"},
		{"123label", false, "key_123label"},
		{"123label", true, "123label"}, // UTF-8 allows numbers at the beginning
		{"_label", false, "key_label"},
		{"_label", true, "_label"}, // UTF-8 allows single underscores at the beginning
		{"__label", false, "__label"},
	}

	for _, test := range tests {
		result := NormalizeLabel(test.label, test.allowUTF8)
		require.Equal(t, test.expected, result)
	}
}
