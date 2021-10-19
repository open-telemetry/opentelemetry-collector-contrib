// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statsreader

import "time"

type timestampsGenerator struct {
	backfillEnabled bool
	difference      time.Duration
}

// This slice will always contain at least one value - now shifted to the start of minute(upper bound).
// In case lastPullTimestamp is greater than now argument slice will contain only one value - now shifted to the start of minute(upper bound).
func (g *timestampsGenerator) pullTimestamps(lastPullTimestamp time.Time, now time.Time) []time.Time {
	var timestamps []time.Time
	upperBound := shiftToStartOfMinute(now)

	if lastPullTimestamp.IsZero() {
		if g.backfillEnabled {
			timestamps = pullTimestampsWithDifference(upperBound.Add(-1*backfillIntervalDuration), upperBound,
				g.difference)
		} else {
			timestamps = []time.Time{upperBound}
		}
	} else {
		// lastPullTimestamp is already set to start of minute
		timestamps = pullTimestampsWithDifference(lastPullTimestamp, upperBound, g.difference)
	}

	return timestamps
}

// This slice will always contain at least one value(upper bound).
// Difference between each two points is 1 minute.
func pullTimestampsWithDifference(lowerBound time.Time, upperBound time.Time, difference time.Duration) []time.Time {
	var timestamps []time.Time

	for value := lowerBound.Add(difference); !value.After(upperBound); value = value.Add(difference) {
		timestamps = append(timestamps, value)
	}

	// To ensure that we did not miss upper bound and timestamps slice will contain at least one value
	if len(timestamps) <= 0 || timestamps[len(timestamps)-1] != upperBound {
		timestamps = append(timestamps, upperBound)
	}

	return timestamps
}

func shiftToStartOfMinute(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
}
