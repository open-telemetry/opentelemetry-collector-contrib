// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"time"
)

// TimeWindow represents a time window boundary (no aggregators stored here)
type TimeWindow struct {
	start time.Time
	end   time.Time
}

// NewTimeWindow creates a new time window boundary
func NewTimeWindow(start, end time.Time) *TimeWindow {
	return &TimeWindow{
		start: start,
		end:   end,
	}
}

// Contains checks if a timestamp falls within this time window
func (tw *TimeWindow) Contains(timestamp time.Time) bool {
	return !timestamp.Before(tw.start) && timestamp.Before(tw.end)
}

// Helper function to parse series key
func parseSeriesKey(seriesKey string) (string, map[string]string) {
	// Format: metricName|label1=value1,label2=value2

	labels := make(map[string]string)

	// Find the pipe separator
	pipeIdx := -1
	for i, ch := range seriesKey {
		if ch == '|' {
			pipeIdx = i
			break
		}
	}

	var metricName string
	var labelsPart string

	if pipeIdx == -1 {
		// No pipe found, entire key is the metric name
		metricName = seriesKey
		return metricName, labels
	}

	// Split metric name and labels
	metricName = seriesKey[:pipeIdx]
	if pipeIdx < len(seriesKey)-1 {
		labelsPart = seriesKey[pipeIdx+1:]
	}

	// Parse labels if present
	if labelsPart != "" {
		// Split by comma
		labelPairs := splitLabels(labelsPart)
		for _, pair := range labelPairs {
			// Split by equals
			eqIdx := -1
			for i, ch := range pair {
				if ch == '=' {
					eqIdx = i
					break
				}
			}

			if eqIdx > 0 && eqIdx < len(pair)-1 {
				key := pair[:eqIdx]
				value := pair[eqIdx+1:]
				labels[key] = value
			}
		}
	}

	return metricName, labels
}

// splitLabels splits label string by comma
func splitLabels(s string) []string {
	var result []string
	var current string

	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
