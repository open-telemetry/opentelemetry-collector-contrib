// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cms // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cms"

import (
	"errors"
	"time"
)

var (
	errIncorrectBucketsCount     = errors.New("incorrect buckets count")
	errEmptyObservInterval       = errors.New("empty observ interval")
	errNonMultiplyObservInterval = errors.New("bucket interval must be a multiple of a second")
)

// BucketsCfg configuration options for the buckets that make up the CMS
// sliding window.
type BucketsCfg struct {
	// ObservationInterval time interval (duration) of a sliding window.
	ObservationInterval time.Duration
	// Buckets nuber of segments (buckets) in sliding window.
	Buckets uint8
	// EstimationSoftLimit the soft limit for each estimation
	EstimationSoftLimit uint32
}

// BucketInterval calculate bucket interval. If the interval is not multiple of
// a second, then an error is returned.
func (cfg BucketsCfg) BucketInterval() (time.Duration, error) {
	bucketInterval := cfg.ObservationInterval / time.Duration(cfg.Buckets)

	if ((cfg.ObservationInterval % time.Duration(cfg.Buckets)) != time.Duration(0)) ||
		(bucketInterval < time.Second) ||
		(bucketInterval%time.Second != 0) {
		return 0, errNonMultiplyObservInterval
	}

	return bucketInterval, nil
}

// Validate validates buckets cfg.
func (cfg BucketsCfg) Validate() error {
	if cfg.ObservationInterval == 0 {
		return errEmptyObservInterval
	}
	if cfg.Buckets == 0 {
		return errIncorrectBucketsCount
	}

	return nil
}

// SlidingCms Sliding window of CMS segments (buckets).
type SlidingCms struct {
	// cmsBuckets ring buffer containing CMS segments (buckets). Each segment
	// contains data for a time period equal to bucketInterval.
	cmsBuckets *RingBufferQueue[CountMinSketch]
	// startPoint start time of the sliding time window. This time point is
	// shifted every bucketInterval.
	startPoint time.Time
	// bucketInterval time interval of each bucket.
	bucketInterval time.Duration
	// softLimit
	softLimit uint32
}

func (h *SlidingCms) shouldUpdateIntervals(tp time.Time) bool {
	return h.startPoint.Add(time.Duration(h.cmsBuckets.Size()) * h.bucketInterval).Before(tp)
}

// CurrentObservationIntervalStartTm returns the current start time of sliding
// window.
func (h *SlidingCms) CurrentObservationIntervalStartTm() time.Time {
	return h.startPoint
}

func (h *SlidingCms) updateBuckets(tp time.Time) {
	if !h.shouldUpdateIntervals(tp) {
		return
	}

	if h.cmsBuckets.IsFull() {
		firstBucket, _ := h.cmsBuckets.Dequeue()
		firstBucket.Clear()

		h.startPoint = h.startPoint.Add(h.bucketInterval * (tp.Sub(h.startPoint) / h.bucketInterval))
	}

	_ = h.cmsBuckets.TailMoveForward()
}

func (h *SlidingCms) insertWithTime(data []byte, tm time.Time) {
	h.updateBuckets(tm)
	t, _ := h.cmsBuckets.Tail()
	t.Insert(data)
}

func (h *SlidingCms) insertCountWithTime(data []byte, tm time.Time) uint32 {
	h.updateBuckets(tm)
	t, _ := h.cmsBuckets.Tail()
	return t.InsertWithCount(data)
}

// InsertWithCount inserts the element into sliding window and returns the
// frequency estimation.
func (h *SlidingCms) InsertWithCount(element []byte) uint32 {
	return h.timedInsertWithCount(element, time.Now())
}

func (h *SlidingCms) timedInsertWithCount(element []byte, tm time.Time) uint32 {
	val := h.insertCountWithTime(element, tm)
	if !h.isUnderLimit(val) {
		return val
	}

	for i := 0; i < h.cmsBuckets.Size()-1; i++ {
		cmsBucket, _ := h.cmsBuckets.At(i)
		val += cmsBucket.Count(element)
		if !h.isUnderLimit(val) {
			return val
		}
	}
	return val
}

func (h *SlidingCms) isUnderLimit(val uint32) bool {
	return h.softLimit == 0 || h.softLimit > val
}

// Count estimates the frequency of occurrences of an element in a sliding window.
func (h *SlidingCms) Count(data []byte) uint32 {
	var val uint32
	h.cmsBuckets.Visit(func(cms CountMinSketch) bool {
		val += cms.Count(data)
		return h.isUnderLimit(val)
	})

	return val
}

// Insert inserts the element into the sliding window.
func (h *SlidingCms) Insert(data []byte) {
	h.insertWithTime(data, time.Now())
}

// Buckets returns the ring buffer of sliding window buckets.
func (h *SlidingCms) Buckets() *RingBufferQueue[CountMinSketch] {
	return h.cmsBuckets
}

// Clear resets cms data for each bucket.
func (h *SlidingCms) Clear() {
	h.cmsBuckets.Visit(func(c CountMinSketch) bool {
		c.Clear()
		return true
	})
}

func NewSlidingCMSWithStartPoint(bucketsCfg BucketsCfg, cmsCfg CountMinSketchCfg, startTm time.Time) (*SlidingCms, error) {
	err := bucketsCfg.Validate()
	if err != nil {
		return nil, err
	}

	bucketInterval, err := bucketsCfg.BucketInterval()
	if err != nil {
		return nil, err
	}

	emptyCmsBuckets := make([]CountMinSketch, 0)
	for i := uint8(0); i < bucketsCfg.Buckets; i++ {
		emptyCmsBuckets = append(emptyCmsBuckets, NewCMSWithErrorParams(&cmsCfg))
	}

	buckets, err := NewRingBufferQueue[CountMinSketch](emptyCmsBuckets)
	_ = buckets.TailMoveForward()

	if err != nil {
		return nil, err
	}

	return &SlidingCms{
		cmsBuckets:     buckets,
		startPoint:     startTm,
		bucketInterval: bucketInterval,
		softLimit:      bucketsCfg.EstimationSoftLimit,
	}, nil
}

func NewSlidingPredefinedCMSWithStartPoint(bucketsCfg BucketsCfg, cmsBuckets []CountMinSketch, startTm time.Time) (*SlidingCms, error) {
	err := bucketsCfg.Validate()
	if err != nil {
		return nil, err
	}

	bucketInterval, err := bucketsCfg.BucketInterval()
	if err != nil {
		return nil, err
	}

	if bucketsCfg.Buckets != uint8(len(cmsBuckets)) {
		return nil, errIncorrectBucketsCount
	}

	buckets, err := NewRingBufferQueue[CountMinSketch](cmsBuckets)
	_ = buckets.TailMoveForward()

	if err != nil {
		return nil, err
	}

	return &SlidingCms{
		cmsBuckets:     buckets,
		startPoint:     startTm,
		bucketInterval: bucketInterval,
		softLimit:      bucketsCfg.EstimationSoftLimit,
	}, nil
}
