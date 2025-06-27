// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"math/rand"
	"sync"
	"time"

	"github.com/lightstep/varopt"
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

// bucketManager implements Varopt reservoir sampling with time-aligned buckets
// as specified in the PRD for OTEP 235/250 compliance using the Varopt library.
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

// traceBucket represents a time-aligned bucket containing traces using Varopt sampling
type traceBucket struct {
	startTime time.Time
	endTime   time.Time
	sampler   *varopt.Varopt[*bucketTrace]
	mutex     sync.RWMutex
}

// bucketTrace represents a trace within a bucket with its randomness value
type bucketTrace struct {
	traceID         pcommon.TraceID
	randomnessValue uint64 // 56 bits from TraceID for sorting
	trace           *internalsampling.TraceData
}

// newBucketManager creates a new bucket manager for Varopt sampling
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
			sampler:   varopt.New[*bucketTrace](int(maxTracesPerBucket), rand.New(rand.NewSource(time.Now().UnixNano()))),
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

	// Use Varopt to add the trace with weight 1.0 (uniform weight for counting case)
	// Note: ejected item and error are ignored for now
	_, err := bucket.sampler.Add(bucketTrace, 1.0)
	if err != nil {
		bm.logger.Warn("Failed to add trace to Varopt sampler", zap.Error(err))
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

// performBottomKCompaction is no longer needed as Varopt handles reservoir sampling automatically

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
				sampler:   varopt.New[*bucketTrace](int(bm.maxTracesPerBucket), rand.New(rand.NewSource(time.Now().UnixNano()))),
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

	// Get all samples from Varopt
	count := bucket.sampler.Size()
	traces := make([]*bucketTrace, 0, count)
	
	for i := 0; i < count; i++ {
		trace, _ := bucket.sampler.Get(i) // Get returns (item, adjustedWeight)
		traces = append(traces, trace)
	}
	
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

// calculateBottomKThreshold calculates the new threshold using Varopt's weight adjustment
// This leverages the Varopt library's built-in weighted sampling capabilities
func (bm *bucketManager) calculateBottomKThreshold(bucket *traceBucket) sampling.Threshold {
	bucket.mutex.RLock()
	defer bucket.mutex.RUnlock()

	count := bucket.sampler.Size()
	if count == 0 {
		bm.logger.Debug("calculateBottomKThreshold: no traces in bucket, returning AlwaysSampleThreshold")
		return sampling.AlwaysSampleThreshold
	}

	// Use Varopt's tau (threshold) to determine sampling probability
	// Tau represents the boundary between large and small weights
	tau := bucket.sampler.Tau()
	totalWeight := bucket.sampler.TotalWeight()
	
	bm.logger.Debug("calculateBottomKThreshold: Varopt statistics",
		zap.Float64("tau", tau),
		zap.Float64("total_weight", totalWeight),
		zap.Int("sample_count", count))

	// For uniform weights (all traces have weight 1.0), the sampling probability
	// is approximately the reservoir size divided by the total weight seen
	samplingProbability := float64(bm.maxTracesPerBucket) / totalWeight
	
	// Ensure probability is within valid range
	if samplingProbability <= 0.0 {
		bm.logger.Debug("calculateBottomKThreshold: sampling probability <= 0, returning NeverSampleThreshold")
		return sampling.NeverSampleThreshold
	}
	if samplingProbability >= 1.0 {
		bm.logger.Debug("calculateBottomKThreshold: sampling probability >= 1.0, returning AlwaysSampleThreshold")
		return sampling.AlwaysSampleThreshold
	}

	// Convert probability to OTEP 235 threshold using pkg/sampling helpers
	threshold, err := sampling.ProbabilityToThreshold(samplingProbability)
	if err != nil {
		bm.logger.Warn("Failed to convert Varopt probability to threshold",
			zap.Float64("probability", samplingProbability),
			zap.Float64("total_weight", totalWeight),
			zap.Int("count", count),
			zap.Error(err))
		// Fallback to a conservative threshold
		return sampling.NeverSampleThreshold
	}

	bm.logger.Debug("Calculated Varopt-based threshold",
		zap.Int("sample_count", count),
		zap.Float64("total_weight", totalWeight),
		zap.Float64("sampling_probability", samplingProbability),
		zap.String("threshold_tvalue", threshold.TValue()))

	return threshold
}

// calculateBottomKThresholdCompat provides backward compatibility for the old Bottom-K interface
// This is a temporary method while we transition to full Varopt integration
func (bm *bucketManager) calculateBottomKThresholdCompat(minRandomnessValueOfKeptTraces uint64, k uint64) sampling.Threshold {
	bm.logger.Debug("calculateBottomKThresholdCompat: Using compatibility method",
		zap.Uint64("minRandomnessValueOfKeptTraces", minRandomnessValueOfKeptTraces),
		zap.Uint64("k", k))

	// For compatibility, we'll calculate a rough threshold based on the old parameters
	// This isn't as accurate as the Varopt-based calculation but provides continuity
	maxRandomness := uint64(0x00FFFFFFFFFFFFFF) // 56-bit max value for OTEP 235

	// Handle edge cases
	if k == 0 {
		bm.logger.Debug("calculateBottomKThresholdCompat: k=0, returning NeverSampleThreshold")
		return sampling.NeverSampleThreshold
	}
	if minRandomnessValueOfKeptTraces == 0 {
		bm.logger.Debug("calculateBottomKThresholdCompat: minRandomness=0, returning AlwaysSampleThreshold")
		return sampling.AlwaysSampleThreshold
	}

	// Simple probability calculation: k / total_space_represented_by_min_randomness
	// In bottom-K sampling, smaller min randomness means we've seen more data
	// The sampling probability is proportional to k / estimated_total_items
	// estimated_total_items ≈ k * (maxRandomness / minRandomness)
	normalizedRandomness := float64(minRandomnessValueOfKeptTraces) / float64(maxRandomness)
	
	// Rough sampling probability estimate: the smaller the min randomness, the higher the probability
	// So we use the inverse relationship, but avoid exact 0.0 and 1.0 to prevent special threshold values
	baseProb := 1.0 - normalizedRandomness
	
	// Clamp to avoid exactly 0.0 or 1.0
	epsilon := 1e-10
	var samplingProbability float64
	if baseProb <= epsilon {
		samplingProbability = epsilon
	} else if baseProb >= 1.0-epsilon {
		samplingProbability = 1.0 - epsilon
	} else {
		samplingProbability = baseProb
	}
	
	// Ensure probability is within valid range
	if samplingProbability <= 0.0 {
		bm.logger.Debug("calculateBottomKThresholdCompat: sampling probability <= 0, returning NeverSampleThreshold")
		return sampling.NeverSampleThreshold
	}
	if samplingProbability >= 1.0 {
		bm.logger.Debug("calculateBottomKThresholdCompat: sampling probability >= 1.0, returning AlwaysSampleThreshold")
		return sampling.AlwaysSampleThreshold
	}

	// Convert probability to OTEP 235 threshold
	threshold, err := sampling.ProbabilityToThreshold(samplingProbability)
	if err != nil {
		bm.logger.Warn("Failed to convert compatibility probability to threshold",
			zap.Float64("probability", samplingProbability),
			zap.Uint64("k", k),
			zap.Uint64("min_randomness", minRandomnessValueOfKeptTraces),
			zap.Error(err))
		return sampling.NeverSampleThreshold
	}

	bm.logger.Debug("Calculated compatibility threshold",
		zap.Uint64("k", k),
		zap.Uint64("min_randomness", minRandomnessValueOfKeptTraces),
		zap.Float64("sampling_probability", samplingProbability),
		zap.String("threshold_tvalue", threshold.TValue()))

	return threshold
}
