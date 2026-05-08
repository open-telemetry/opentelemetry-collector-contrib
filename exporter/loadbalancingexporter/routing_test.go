// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func findRoutingIDForEndpoint(t testing.TB, ring *hashRing, endpoint string) string {
	t.Helper()

	for i := range 4096 {
		routingID := fmt.Sprintf("routing-id-%d", i)
		ringEndpoint := ring.endpointFor([]byte(routingID))
		if ringEndpoint == endpoint || endpointWithPort(ringEndpoint) == endpointWithPort(endpoint) {
			return routingID
		}
	}

	require.FailNow(t, "failed to find routing id for endpoint", "endpoint=%s", endpoint)
	return ""
}
