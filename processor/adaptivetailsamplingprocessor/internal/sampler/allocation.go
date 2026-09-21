// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"math"
	"sort"
	"time"
)

// This file ports the EMA throughput rate math from dynsampler-go v0.6.4
// (EMAThroughput.updateMaps, EMAThroughput.updateEMA, adjustAverage, and
// calculateSampleRates) so rates can be computed over externally merged
// counts, which the library cannot ingest. Golden tests in allocation_test.go
// pin parity against the library's own test vectors. Burst detection is
// deliberately not ported: the shared-counter sync runs on fixed interval
// ticks, so there is no early-adjustment path to trigger.

// emaState holds the per-key exponential moving averages a throughput sampler
// derives from a sequence of per-interval raw counts. It is not safe for
// concurrent use; callers serialize access.
type emaState struct {
	movingAverage map[string]float64
	// weight is the EMA alpha in (0, 1): larger values adapt faster.
	weight float64
	// ageOutValue drops keys whose average decays below it, bounding the map.
	ageOutValue float64
	// maxKeys bounds the tracked keys; 0 means unbounded. New keys beyond
	// the cap are dropped at observe time in sorted order, so instances
	// applying identical merged counts stay in agreement; age-out frees
	// slots. Keys beyond the cap fall back to the caller's bootstrap rate,
	// unlike dynsampler's keep-everything overflow (see contrib #50538).
	maxKeys int
}

// newEMAState mirrors dynsampler-go's defaults: weight 0.5 when unset, and
// ageOutValue defaulting to the weight.
func newEMAState(weight float64, maxKeys int) *emaState {
	if weight == 0 {
		weight = 0.5
	}
	return &emaState{
		movingAverage: make(map[string]float64),
		weight:        weight,
		ageOutValue:   weight,
		maxKeys:       maxKeys,
	}
}

// observe folds one completed interval of raw counts into the moving
// averages, consuming counts. An empty interval leaves the averages untouched
// (mirroring the library: traffic gaps must not decay the averages).
func (s *emaState) observe(counts map[string]float64) {
	if len(counts) == 0 {
		return
	}
	keysToUpdate := make([]string, 0, len(s.movingAverage))
	for key := range s.movingAverage {
		keysToUpdate = append(keysToUpdate, key)
	}
	for _, key := range keysToUpdate {
		var newAvg float64
		if val, found := counts[key]; found {
			newAvg = adjustAverage(s.movingAverage[key], val, s.weight)
		} else {
			newAvg = adjustAverage(s.movingAverage[key], 0, s.weight)
		}
		if newAvg < s.ageOutValue {
			delete(s.movingAverage, key)
		} else {
			s.movingAverage[key] = newAvg
		}
		delete(counts, key)
	}
	newKeys := make([]string, 0, len(counts))
	for key := range counts {
		newKeys = append(newKeys, key)
	}
	sort.Strings(newKeys)
	for _, key := range newKeys {
		if s.maxKeys > 0 && len(s.movingAverage) >= s.maxKeys {
			break
		}
		newAvg := adjustAverage(0, counts[key], s.weight)
		if newAvg >= s.ageOutValue {
			s.movingAverage[key] = newAvg
		}
	}
}

// rates computes per-key sample rates targeting goalThroughputPerSec over the
// adjustment interval. Returns nil when no averages exist yet, so callers keep
// their bootstrap rates until the first non-empty interval is observed.
func (s *emaState) rates(goalThroughputPerSec float64, interval time.Duration) map[string]int {
	if len(s.movingAverage) == 0 {
		return nil
	}
	goalCount := goalThroughputPerSec * interval.Seconds()
	var logSum float64
	for _, count := range s.movingAverage {
		// max(1, count) because count*weight can be < 1 for very small
		// counts, which throws off the logSum at low throughput.
		logSum += math.Log10(math.Max(1, count))
	}
	goalRatio := goalCount / logSum
	return calculateSampleRates(goalRatio, s.movingAverage)
}

// calculateSampleRates is a verbatim port of dynsampler-go v0.6.4's shared
// key-allocation logic: each key gets a log10 share of the goal, keys under
// their share are kept whole (rate 1) and pass their surplus to the remaining
// keys, iterated in sorted order so rounding is deterministic.
func calculateSampleRates(goalRatio float64, buckets map[string]float64) map[string]int {
	keys := make([]string, len(buckets))
	var i int
	for k := range buckets {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	newSampleRates := make(map[string]int)
	keysRemaining := len(buckets)
	var extra float64
	for _, key := range keys {
		count := math.Max(1, buckets[key])
		goalForKey := math.Max(1, math.Log10(count)*goalRatio)
		extraForKey := extra / float64(keysRemaining)
		goalForKey += extraForKey
		extra -= extraForKey
		keysRemaining--
		if count <= goalForKey {
			newSampleRates[key] = 1
			extra += goalForKey - count
		} else {
			rate := math.Ceil(count / goalForKey)
			// goalForKey can be +Inf for tiny counts, making rate NaN.
			if math.IsNaN(rate) {
				newSampleRates[key] = 1
			} else {
				newSampleRates[key] = int(rate)
			}
			extra += goalForKey - (count / float64(newSampleRates[key]))
		}
	}
	return newSampleRates
}

// adjustAverage folds value into oldAvg with EMA weighting alpha.
func adjustAverage(oldAvg, value, alpha float64) float64 {
	return value*alpha + (1.0-alpha)*oldAvg
}
