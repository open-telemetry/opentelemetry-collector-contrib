// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package valid

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostname(t *testing.T) {
	// Empty
	require.Error(t, Hostname(""))

	// Local
	require.Error(t, Hostname("localhost"))

	// Max length exceeded
	require.Error(t, Hostname(strings.Repeat("a", 256)))

	// non RFC1123 compliant
	require.Error(t, Hostname("***"))
	require.Error(t, Hostname("invalid_hostname"))

	require.NoError(t, Hostname("valid-hostname"))
}
