// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearch // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/integrationtests/elasticsearch"

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func getDebugFlag(tb testing.TB) bool {
	raw := os.Getenv("DEBUG")
	if raw == "" {
		return false
	}
	debug, err := strconv.ParseBool(raw)
	require.NoError(tb, err, "debug flag parsing failed")
	return debug
}
