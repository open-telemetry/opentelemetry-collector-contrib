// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package networkscraper

import (
	"testing"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/stretchr/testify/assert"
)

// TestGetTCPConnectionStatusCountsMatchesGopsutilLinuxStateNames guards against the
// state name strings in allTCPStates drifting from what gopsutil's Linux
// implementation actually reports (e.g. "FIN_WAIT1", not "FIN_WAIT_1" - that's the
// Windows spelling). A mismatch doesn't drop real connections (map indexing still
// works via the real, correctly-spelled key), but it does mean the mismatched state
// never gets an explicit zero when idle, and the wrongly-spelled key sits in the
// map permanently pinned at zero.
func TestGetTCPConnectionStatusCountsMatchesGopsutilLinuxStateNames(t *testing.T) {
	connections := []net.ConnectionStat{
		{Status: "FIN_WAIT1"},
		{Status: "FIN_WAIT1"},
		{Status: "FIN_WAIT2"},
		{Status: "ESTABLISHED"},
	}

	counts := getTCPConnectionStatusCounts(connections)

	assert.Equal(t, int64(2), counts["FIN_WAIT1"])
	assert.Equal(t, int64(1), counts["FIN_WAIT2"])
	assert.Equal(t, int64(1), counts["ESTABLISHED"])

	// Every other known state should still be pre-seeded at zero.
	assert.Equal(t, int64(0), counts["LISTEN"])
	assert.Equal(t, int64(0), counts["TIME_WAIT"])

	// The old, wrongly-spelled Windows-style keys must not appear at all.
	_, hasUnderscoreVariant1 := counts["FIN_WAIT_1"]
	_, hasUnderscoreVariant2 := counts["FIN_WAIT_2"]
	assert.False(t, hasUnderscoreVariant1, "FIN_WAIT_1 (Windows spelling) should not appear in Linux TCP state counts")
	assert.False(t, hasUnderscoreVariant2, "FIN_WAIT_2 (Windows spelling) should not appear in Linux TCP state counts")

	assert.Len(t, counts, len(allTCPStates), "no extra, unexpected keys should be present")
}
