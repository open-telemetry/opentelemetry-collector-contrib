// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import "time"

// statementCounters stores all cumulative counters for one statement together.
type statementCounters map[string]float64

// statementSnapshot associates a baseline with the global reset it followed.
// A zero statsReset means reset information was unavailable.
type statementSnapshot struct {
	counters   statementCounters
	statsReset time.Time
}

// statementDelta calculates interval counters. An unseen statement seeds a baseline.
// A changed reset timestamp or a decreased counter resets the whole statement.
// The caller must always cache current, including zero values.
func statementDelta(previous, current statementSnapshot) statementCounters {
	deltas := make(statementCounters, len(current.counters))
	if previous.counters == nil {
		return deltas
	}
	if !previous.statsReset.IsZero() && !current.statsReset.IsZero() && !previous.statsReset.Equal(current.statsReset) {
		return current.counters
	}
	for column, value := range current.counters {
		if value < previous.counters[column] {
			return current.counters
		}
	}
	for column, value := range current.counters {
		deltas[column] = value - previous.counters[column]
	}
	return deltas
}
