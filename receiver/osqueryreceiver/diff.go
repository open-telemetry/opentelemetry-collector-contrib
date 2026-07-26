// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import "maps"

// diffRows returns the rows in current that are either not present in
// previous (by rowKey) or present but with different column values. Rows
// present in previous but absent from current are not reported: this receiver
// surfaces new and modified rows only, not removals.
func diffRows(rowKey func(map[string]string) string, previous, current []map[string]string) []map[string]string {
	previousByKey := make(map[string]map[string]string, len(previous))
	for _, row := range previous {
		previousByKey[rowKey(row)] = row
	}

	var changed []map[string]string
	for _, row := range current {
		previousRow, ok := previousByKey[rowKey(row)]
		if !ok || !maps.Equal(previousRow, row) {
			changed = append(changed, row)
		}
	}
	return changed
}
