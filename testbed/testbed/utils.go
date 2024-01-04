// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func GetAvailablePort(t testing.TB) int {
	endpoint := testutil.GetAvailableLocalAddress(t)
	_, port, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)

	portInt, err := strconv.Atoi(port)
	require.NoError(t, err)

	return portInt
}
