// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileset // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

var errFilesetEmpty = errors.New("pop() on empty Fileset")

var (
	_ Matchable = (*reader.Reader)(nil)
	_ Matchable = (*reader.Metadata)(nil)
)

type CompareMode int

const (
	CompareModeEqual CompareMode = iota
	CompareModeStartsWith
	CompareModeCustom
)

type Matchable interface {
	GetFingerprint() *fingerprint.Fingerprint
}

type bucket struct {
	indices []int
}

type bucketPos struct {
	bucket *bucket
	idx    int
}

type Fileset[T Matchable] struct {
	readers   []T
	positions []bucketPos
	buckets   map[int]map[string]*bucket
}

func New[T Matchable](capacity int) *Fileset[T] {
	return &Fileset[T]{
		readers:   make([]T, 0, capacity),
		positions: make([]bucketPos, 0, capacity),
		buckets:   make(map[int]map[string]*bucket),
	}
}

func (set *Fileset[T]) Len() int {
	return len(set.readers)
}

func (set *Fileset[T]) Get() []T {
	return set.readers
}

// Pop removes and returns the most recently added element.
func (set *Fileset[T]) Pop() (T, error) {
	var val T
	if len(set.readers) == 0 {
		return val, errFilesetEmpty
	}
	return set.removeAt(len(set.readers) - 1), nil
}

func (set *Fileset[T]) Add(readers ...T) {
	for _, reader := range readers {
		set.readers = append(set.readers, reader)
		idx := len(set.readers) - 1
		pos := bucketPos{}

		if fp := reader.GetFingerprint(); fp != nil && fp.Len() > 0 {
			pos = set.insertIntoBucket(idx, fp)
		}
		set.positions = append(set.positions, pos)
	}
}

// Reset clears the fileset while retaining allocated buffers.
func (set *Fileset[T]) Reset() {
	var zero T
	for i := range set.readers {
		set.readers[i] = zero
	}
	set.readers = set.readers[:0]
	for i := range set.positions {
		set.positions[i] = bucketPos{}
	}
	set.positions = set.positions[:0]
	for _, buckets := range set.buckets {
		for _, b := range buckets {
			b.indices = b.indices[:0]
		}
	}
}

func (set *Fileset[T]) Match(fp *fingerprint.Fingerprint, mode CompareMode) T {
	switch mode {
	case CompareModeEqual:
		return set.matchEqual(fp)
	case CompareModeStartsWith:
		return set.matchStartsWith(fp)
	default:
		return set.matchLinear(fp, mode)
	}
}

func (set *Fileset[T]) insertIntoBucket(idx int, fp *fingerprint.Fingerprint) bucketPos {
	keyLen := fp.Len()
	prefix := fp.Key()

	if len(prefix) > keyLen {
		prefix = prefix[:keyLen]
	}

	lengthBuckets := set.buckets[keyLen]
	if lengthBuckets == nil {
		// Preallocate for common fingerprint lengths to reduce map allocations.
		// Most files will have full-sized fingerprints (1000 bytes by default).
		capacity := 8
		if keyLen >= 990 && keyLen <= 1010 {
			capacity = 64 // Common case: files with ~1000 byte fingerprints
		}
		lengthBuckets = make(map[string]*bucket, capacity)
		set.buckets[keyLen] = lengthBuckets
	}

	b := lengthBuckets[prefix]
	if b == nil {
		b = &bucket{}
		lengthBuckets[prefix] = b
	}
	b.indices = append(b.indices, idx)
	return bucketPos{
		bucket: b,
		idx:    len(b.indices) - 1,
	}
}

func (set *Fileset[T]) removeAt(idx int) T {
	val := set.readers[idx]
	set.removeFromBucket(idx)

	lastIdx := len(set.readers) - 1
	if idx != lastIdx {
		set.readers[idx] = set.readers[lastIdx]
		set.positions[idx] = set.positions[lastIdx]
		set.updateBucketPosition(idx)
	}
	var zero T
	set.readers[lastIdx] = zero
	set.readers = set.readers[:lastIdx]
	set.positions[lastIdx] = bucketPos{}
	set.positions = set.positions[:lastIdx]
	return val
}

func (set *Fileset[T]) removeFromBucket(idx int) {
	pos := set.positions[idx]
	if pos.bucket == nil {
		return
	}
	last := len(pos.bucket.indices) - 1
	swapIdx := pos.bucket.indices[last]
	pos.bucket.indices[pos.idx] = swapIdx
	pos.bucket.indices = pos.bucket.indices[:last]
	if pos.idx < last {
		set.positions[swapIdx].idx = pos.idx
	}
	set.positions[idx] = bucketPos{}
}

func (set *Fileset[T]) updateBucketPosition(idx int) {
	pos := &set.positions[idx]
	if pos.bucket == nil {
		return
	}
	pos.bucket.indices[pos.idx] = idx
}

func (set *Fileset[T]) matchEqual(fp *fingerprint.Fingerprint) T {
	var zero T
	if fp == nil || fp.Len() == 0 {
		return zero
	}
	fpLen := fp.Len()
	fpKey := fp.Key()
	if bucketsByLength, ok := set.buckets[fpLen]; ok {
		if bucket := bucketsByLength[fpKey]; bucket != nil {
			for i := len(bucket.indices) - 1; i >= 0; i-- {
				idx := bucket.indices[i]
				if fp.Equal(set.readers[idx].GetFingerprint()) {
					return set.removeAt(idx)
				}
			}
		}
	}
	return zero
}

func (set *Fileset[T]) matchStartsWith(fp *fingerprint.Fingerprint) T {
	var zero T
	if fp == nil || fp.Len() == 0 {
		return zero
	}
	fpKey := fp.Key()
	fpLen := len(fpKey)
	for length := fpLen; length > 0; length-- {
		bucketsByLength, ok := set.buckets[length]
		if !ok || length > len(fpKey) {
			continue
		}
		prefix := fpKey[:length]
		if bucket := bucketsByLength[prefix]; bucket != nil {
			for i := len(bucket.indices) - 1; i >= 0; i-- {
				idx := bucket.indices[i]
				if fp.StartsWith(set.readers[idx].GetFingerprint()) {
					return set.removeAt(idx)
				}
			}
		}
	}
	return zero
}

func (set *Fileset[T]) matchLinear(fp *fingerprint.Fingerprint, mode CompareMode) T {
	var zero T
	for idx, r := range set.readers {
		var matches bool
		switch mode {
		case CompareModeEqual:
			matches = fp.Equal(r.GetFingerprint())
		case CompareModeStartsWith:
			matches = fp.StartsWith(r.GetFingerprint())
		}
		if matches {
			return set.removeAt(idx)
		}
	}
	return zero
}
