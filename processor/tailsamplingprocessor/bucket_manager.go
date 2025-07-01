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
	traceID             pcommon.TraceID
	randomnessValue     uint64 // 56 bits from TraceID for sorting
	trace               *internalsampling.TraceData
	inputWeight         float64 // OTEP 235 adjusted count used as Varopt input weight
	varopAdjustedWeight float64 // Varopt output weight (set during bucket processing)
}

// newBucketManager creates a new bucket manager for Varopt sampling
func newBucketManager(logger *zap.Logger, bucketCount uint64, decisionWait time.Duration, maxTraces uint64, tracesPerBucketFactor float64) *bucketManager {
	return newBucketManagerWithTimeSource(logger, bucketCount, decisionWait, maxTraces, tracesPerBucketFactor, &realTimeSource{})
}

// newBucketManagerWithTimeSource creates a new bucket manager with injectable time source for testing
func newBucketManagerWithTimeSource(logger *zap.Logger, bucketCount uint64, decisionWait time.Duration, maxTraces uint64, tracesPerBucketFactor float64, timeSource TimeSource) *bucketManager {
	// Handle zero bucket count
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

		// Ensure minimum capacity of 1 to avoid Varopt panics
		capacity := int(maxTracesPerBucket)
		if capacity <= 0 {
			capacity = 1
			bm.logger.Warn("Zero or negative bucket capacity, using minimum capacity of 1",
				zap.Uint64("original_max_traces_per_bucket", maxTracesPerBucket),
				zap.Int("adjusted_capacity", capacity))
		}

		bm.buckets[i] = &traceBucket{
			startTime: bucketStart,
			endTime:   bucketEnd,
			sampler:   varopt.New[*bucketTrace](capacity, rand.New(rand.NewSource(time.Now().UnixNano()))),
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

	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()

	// Use OTEP 235 adjusted count as the weight for Varopt
	// This ensures heavily-sampled traces are more likely to remain in the reservoir
	var inputWeight float64 = 1.0   // Default weight for uniform sampling
	var thresholdFound bool = false // Track whether we found an OTEP 235 threshold

	// Extract threshold from the first span's TraceState to get OTEP 235 adjusted count
	resourceSpans := trace.ReceivedBatches.ResourceSpans()
	for i := 0; i < resourceSpans.Len() && !thresholdFound; i++ {
		scopeSpans := resourceSpans.At(i).ScopeSpans()
		for j := 0; j < scopeSpans.Len() && !thresholdFound; j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len() && !thresholdFound; k++ {
				span := spans.At(k)
				traceState := span.TraceState()
				if traceState.AsRaw() != "" {
					// Parse the W3C TraceState to extract threshold
					w3cTS, err := sampling.NewW3CTraceState(traceState.AsRaw())
					if err == nil {
						otelTS := w3cTS.OTelValue()
						if otelTS != nil {
							if threshold, valid := otelTS.TValueThreshold(); valid {
								inputWeight = threshold.AdjustedCount()
								thresholdFound = true
								break
							}
						}
					}
				}
			}
		}
	}

	bucketTrace := &bucketTrace{
		traceID:         traceID,
		randomnessValue: randomnessValue,
		trace:           trace,
		inputWeight:     inputWeight, // Store the input weight for later adjustment calculation
	}

	// Use Varopt to add the trace with the OTEP 235 adjusted count as weight
	_, err := bucket.sampler.Add(bucketTrace, inputWeight)
	if err != nil {
		bm.logger.Warn("Failed to add trace to Varopt sampler",
			zap.Stringer("traceID", traceID),
			zap.Float64("weight", inputWeight),
			zap.Error(err))
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

			// Ensure minimum capacity of 1 to avoid Varopt panics
			capacity := int(bm.maxTracesPerBucket)
			if capacity <= 0 {
				capacity = 1
			}

			bm.buckets[i] = &traceBucket{
				startTime: newStartTime,
				endTime:   newEndTime,
				sampler:   varopt.New[*bucketTrace](capacity, rand.New(rand.NewSource(time.Now().UnixNano()))),
			}

			return expiredBucket
		}
	}

	return nil
}

// getTracesWithTailAdjustments returns all traces from a bucket with their tail sampling adjustments
// The tail sampling adjustment is the ratio of Varopt output weight to input weight
func (bucket *traceBucket) getTracesWithTailAdjustments() []*bucketTrace {
	bucket.mutex.RLock()
	defer bucket.mutex.RUnlock()

	// Get all samples from Varopt with their adjusted weights
	count := bucket.sampler.Size()
	traces := make([]*bucketTrace, 0, count)

	for i := 0; i < count; i++ {
		trace, varopOutputWeight := bucket.sampler.Get(i) // Get returns (item, adjustedWeight)

		// Calculate the tail sampling adjustment from weight ratios
		// Note: The input weight was the OTEP 235 adjusted count
		// The output weight is Varopt's adjusted weight
		// Tail adjustment = output_weight / input_weight
		// This will be added as sampling.tail.adjusted_count span attribute

		// Store the Varopt output weight in the trace for later use
		// We'll use this when adding span attributes during output
		trace.varopAdjustedWeight = varopOutputWeight

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

// getTailSamplingAdjustment calculates the tail sampling adjustment for a specific trace
// This is the ratio of Varopt output weight to OTEP 235 input weight
func (bt *bucketTrace) getTailSamplingAdjustment() float64 {
	if bt.inputWeight <= 0 {
		return 1.0 // Fallback to no adjustment if input weight is invalid
	}
	return bt.varopAdjustedWeight / bt.inputWeight
}
