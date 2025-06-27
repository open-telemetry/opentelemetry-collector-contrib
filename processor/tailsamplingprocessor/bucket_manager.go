// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"math"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	internalsampling "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

// TimeSource allows time injection for testing
type TimeSource interface {
	Now() time.Time
}

// realTimeSource implements TimeSource using actual system time
type realTimeSource struct{}

func (r *realTimeSource) Now() time.Time {
	return time.Now()
}

// bucketManager implements Bottom-K reservoir sampling with time-aligned buckets
// as specified in the PRD for OTEP 235/250 compliance.
type bucketManager struct {
	logger *zap.Logger

	// Configuration
	bucketCount        uint64
	bucketDuration     time.Duration
	maxTracesPerBucket uint64
	startTime          time.Time
	timeSource         TimeSource

	// Per-bucket trace storage
	buckets []*traceBucket
	mutex   sync.RWMutex
}

// traceBucket represents a time-aligned bucket containing traces
type traceBucket struct {
	startTime time.Time
	endTime   time.Time
	traces    []*bucketTrace
	mutex     sync.RWMutex
}

// bucketTrace represents a trace within a bucket with its randomness value
type bucketTrace struct {
	traceID         pcommon.TraceID
	randomnessValue uint64 // 56 bits from TraceID for sorting
	trace           *internalsampling.TraceData
}

// newBucketManager creates a new bucket manager for Bottom-K sampling
func newBucketManager(logger *zap.Logger, bucketCount uint64, decisionWait time.Duration, maxTraces uint64, tracesPerBucketFactor float64) *bucketManager {
	return newBucketManagerWithTimeSource(logger, bucketCount, decisionWait, maxTraces, tracesPerBucketFactor, &realTimeSource{})
}

// newBucketManagerWithTimeSource creates a new bucket manager with injectable time source for testing
func newBucketManagerWithTimeSource(logger *zap.Logger, bucketCount uint64, decisionWait time.Duration, maxTraces uint64, tracesPerBucketFactor float64, timeSource TimeSource) *bucketManager {
	// Handle zero bucket count for backward compatibility
	if bucketCount == 0 {
		bucketCount = 10 // Default value
	}

	// Handle zero or invalid traces per bucket factor
	if tracesPerBucketFactor <= 0 {
		tracesPerBucketFactor = 1.1 // Default value
	}

	bucketDuration := decisionWait / time.Duration(bucketCount)
	maxTracesPerBucket := uint64(float64(maxTraces/bucketCount) * tracesPerBucketFactor)

	// Align to current time boundary
	now := timeSource.Now()
	alignedStart := now.Truncate(bucketDuration)

	bm := &bucketManager{
		logger:             logger,
		bucketCount:        bucketCount,
		bucketDuration:     bucketDuration,
		maxTracesPerBucket: maxTracesPerBucket,
		startTime:          alignedStart,
		timeSource:         timeSource,
		buckets:            make([]*traceBucket, bucketCount),
	}

	// Initialize buckets with time boundaries
	for i := uint64(0); i < bucketCount; i++ {
		bucketStart := alignedStart.Add(time.Duration(i) * bucketDuration)
		bucketEnd := bucketStart.Add(bucketDuration)

		bm.buckets[i] = &traceBucket{
			startTime: bucketStart,
			endTime:   bucketEnd,
			traces:    make([]*bucketTrace, 0),
		}
	}

	return bm
}

// addTrace adds a trace to the appropriate time-aligned bucket
func (bm *bucketManager) addTrace(traceID pcommon.TraceID, trace *internalsampling.TraceData, arrivalTime time.Time) {
	bucketIndex := bm.getBucketIndex(arrivalTime)
	bucket := bm.buckets[bucketIndex]

	// Extract randomness value from TraceID (lower 56 bits)
	randomnessValue := extractRandomnessValue(traceID)

	bucketTrace := &bucketTrace{
		traceID:         traceID,
		randomnessValue: randomnessValue,
		trace:           trace,
	}

	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()

	bucket.traces = append(bucket.traces, bucketTrace)

	// Apply Bottom-K compaction if bucket exceeds capacity
	if uint64(len(bucket.traces)) > bm.maxTracesPerBucket {
		bm.performBottomKCompaction(bucket)
	}
}

// getBucketIndex returns the bucket index for a given arrival time
func (bm *bucketManager) getBucketIndex(arrivalTime time.Time) uint64 {
	// Safety check for zero bucket duration or count
	if bm.bucketDuration == 0 || bm.bucketCount == 0 {
		return 0
	}
	elapsed := arrivalTime.Sub(bm.startTime)
	bucketIndex := uint64(elapsed/bm.bucketDuration) % bm.bucketCount
	return bucketIndex
}

// performBottomKCompaction applies the Bottom-K algorithm to maintain bucket size limits
func (bm *bucketManager) performBottomKCompaction(bucket *traceBucket) {
	// Sort traces by randomness value (ascending, so we keep the highest values)
	sort.Slice(bucket.traces, func(i, j int) bool {
		return bucket.traces[i].randomnessValue < bucket.traces[j].randomnessValue
	})

	// Keep only the top K+1 traces (highest randomness values)
	keepCount := int(bm.maxTracesPerBucket)
	if len(bucket.traces) > keepCount {
		// Just keep the K+1 traces with highest randomness values
		// Threshold updates are deferred until output for efficiency
		keptTraces := bucket.traces[len(bucket.traces)-keepCount:]
		bucket.traces = keptTraces

		logFields := []zap.Field{
			zap.Int("bucket_traces_before", len(bucket.traces)+keepCount),
			zap.Int("bucket_traces_after", len(bucket.traces)),
		}
		if len(keptTraces) > 0 {
			logFields = append(logFields, zap.Uint64("min_randomness_kept", keptTraces[0].randomnessValue))
		}
		bm.logger.Debug("Performed Bottom-K compaction", logFields...)
	}
}

// getExpiredBucket returns and resets the bucket that should be processed now
func (bm *bucketManager) getExpiredBucket(currentTime time.Time) *traceBucket {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// Find buckets that have expired
	for i, bucket := range bm.buckets {
		if currentTime.After(bucket.endTime) {
			// Return this bucket for processing
			expiredBucket := bucket

			// Reset this bucket for future use
			newStartTime := bucket.endTime
			newEndTime := newStartTime.Add(time.Duration(bm.bucketCount) * bm.bucketDuration)

			bm.buckets[i] = &traceBucket{
				startTime: newStartTime,
				endTime:   newEndTime,
				traces:    make([]*bucketTrace, 0),
			}

			return expiredBucket
		}
	}

	return nil
}

// getAllTraces returns all traces from a bucket for policy evaluation
func (bucket *traceBucket) getAllTraces() []*bucketTrace {
	bucket.mutex.RLock()
	defer bucket.mutex.RUnlock()

	// Return copy to avoid concurrent access issues
	traces := make([]*bucketTrace, len(bucket.traces))
	copy(traces, bucket.traces)
	return traces
}

// extractRandomnessValue extracts the 56-bit randomness value from a TraceID
// as specified in OTEP 235 for consistent probability sampling
func extractRandomnessValue(traceID pcommon.TraceID) uint64 {
	// TraceID is 16 bytes, we use the lower 8 bytes for randomness
	bytes := traceID[8:] // Last 8 bytes

	// Convert to uint64, masking to 56 bits as per OTEP 235
	var value uint64
	for i, b := range bytes {
		value |= uint64(b) << (8 * i)
	}

	// Mask to 56 bits (0x00FFFFFFFFFFFFFF)
	return value & 0x00FFFFFFFFFFFFFF
}

// calculateBottomKThreshold calculates the new threshold using the proper Bottom-K estimator
// This implements the algorithm described in the Bottom-K paper and PRD
func (bm *bucketManager) calculateBottomKThreshold(minRandomnessValueOfKeptTraces uint64, k uint64) sampling.Threshold {
	bm.logger.Debug("calculateBottomKThreshold called",
		zap.Uint64("minRandomnessValueOfKeptTraces", minRandomnessValueOfKeptTraces),
		zap.Uint64("k", k))

	maxRandomness := uint64(0x00FFFFFFFFFFFFFF) // 56-bit max value for OTEP 235

	// Handle edge cases
	if k == 0 {
		bm.logger.Debug("calculateBottomKThreshold: k=0, returning NeverSampleThreshold")
		return sampling.NeverSampleThreshold
	}
	if minRandomnessValueOfKeptTraces == 0 {
		// If minimum randomness is 0, we can't calculate a meaningful threshold
		bm.logger.Debug("calculateBottomKThreshold: minRandomness=0, returning AlwaysSampleThreshold")
		return sampling.AlwaysSampleThreshold
	}

	// Bottom-K threshold calculation using corrected weighted estimator formula:
	// a(i) = w(i) × exp(w(i) × r_{k+1})
	// where r_{k+1} = -ln(R_k) and w(i) is the weight (adjusted count) for this implementation

	// For uniform-weight (counting) case, w(i) = 1.0 (incoming adjusted count)
	incomingWeight := 1.0

	// Convert randomness value to [0,1) range as per OTEP 235
	normalizedRandomness := float64(minRandomnessValueOfKeptTraces) / float64(maxRandomness)

	// Calculate rank: r_{k+1} = -ln(normalized_randomness)
	// Handle edge case where randomness approaches 0
	if normalizedRandomness <= 0.0 {
		normalizedRandomness = 1.0 / float64(maxRandomness) // Minimum non-zero value
	}

	rank := -math.Log(normalizedRandomness) / incomingWeight

	// Calculate adjustment factor using Bottom-K estimator formula
	// For uniform weights: adjustment = w(i) × exp(rank) × k
	adjustmentFactor := incomingWeight * math.Exp(rank) * float64(k)

	// The sampling probability is the inverse of the adjustment factor
	samplingProbability := 1.0 / adjustmentFactor

	bm.logger.Debug("calculateBottomKThreshold: Bottom-K weighted estimator calculation",
		zap.Float64("sampling_probability", samplingProbability),
		zap.Float64("adjustment_factor", adjustmentFactor),
		zap.Float64("rank", rank),
		zap.Float64("normalized_randomness", normalizedRandomness),
		zap.Float64("incoming_weight", incomingWeight),
		zap.Uint64("k", k),
		zap.Uint64("min_randomness", minRandomnessValueOfKeptTraces),
		zap.Uint64("max_randomness", maxRandomness))

	// Edge case: if sampling probability is too low, return never sample
	if samplingProbability <= 0.0 {
		bm.logger.Debug("calculateBottomKThreshold: sampling probability <= 0, returning NeverSampleThreshold")
		return sampling.NeverSampleThreshold
	}

	// Edge case: if sampling probability is 1.0 or higher, return always sample
	if samplingProbability >= 1.0 {
		bm.logger.Debug("calculateBottomKThreshold: sampling probability >= 1.0, returning AlwaysSampleThreshold")
		return sampling.AlwaysSampleThreshold
	}

	// Convert probability to OTEP 235 threshold using pkg/sampling helpers
	threshold, err := sampling.ProbabilityToThreshold(samplingProbability)
	if err != nil {
		bm.logger.Warn("Failed to convert Bottom-K probability to threshold",
			zap.Float64("probability", samplingProbability),
			zap.Uint64("k", k),
			zap.Uint64("min_randomness", minRandomnessValueOfKeptTraces),
			zap.Error(err))
		// Fallback to a conservative threshold
		return sampling.NeverSampleThreshold
	}

	bm.logger.Debug("Calculated Bottom-K threshold",
		zap.Uint64("k", k),
		zap.Uint64("min_randomness", minRandomnessValueOfKeptTraces),
		zap.Float64("sampling_probability", samplingProbability),
		zap.String("threshold_tvalue", threshold.TValue()))

	return threshold
}
