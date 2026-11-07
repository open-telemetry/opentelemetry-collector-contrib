// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package saphanareceiver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// SECONDS_BETWEEN(a, b) returns b - a. The scraper mock matches query text, so it cannot
// notice reversed arguments, which would silently flip the sign of the reported value.
func TestSecondsBetweenArgumentOrder(t *testing.T) {
	tests := []struct {
		table string
		call  string
	}{
		// Time elapsed since the last successful backup.
		{table: "SYS.M_BACKUP_CATALOG", call: "seconds_between(UTC_START_TIME, CURRENT_TIMESTAMP)"},
		// Time elapsed since the database started.
		{table: "SYS.M_DATABASE", call: "seconds_between(START_TIME, CURRENT_TIMESTAMP)"},
		// Time remaining until the license expires.
		{table: "SYS.M_LICENSES", call: "seconds_between(CURRENT_TIMESTAMP, EXPIRATION_DATE)"},
	}

	for _, tt := range tests {
		t.Run(tt.table, func(t *testing.T) {
			found := false
			for _, q := range queries {
				if strings.Contains(q.query, "FROM "+tt.table) {
					found = true
					require.Contains(t, q.query, tt.call)
				}
			}
			require.True(t, found, "no query reads from %s", tt.table)
		})
	}
}
