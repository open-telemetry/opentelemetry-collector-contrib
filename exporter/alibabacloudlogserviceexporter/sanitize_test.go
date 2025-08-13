// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The code for sanitize is mostly copied from:
//  https://github.com/open-telemetry/opentelemetry-collector/blob/2e84285efc665798d76773b9901727e8836e9d8f/exporter/prometheusexporter/sanitize_test.go

func TestSanitize(t *testing.T) {
	require.Empty(t, sanitize(""))
	require.Equal(t, "key_test", sanitize("_test"))
	require.Equal(t, "key_0test", sanitize("0test"))
	require.Equal(t, "test", sanitize("test"))
	require.Equal(t, "test__", sanitize("test_/"))
}
