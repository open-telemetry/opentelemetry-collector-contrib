// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileset // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"

import (
	"errors"
	"slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

var errFilesetEmpty = errors.New("pop() on empty Fileset")

var (
	_ Matchable = (*reader.Reader)(nil)
	_ Matchable = (*reader.Metadata)(nil)
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
	readers       []T
	positions     []bucketPos
	buckets       map[int]map[string]*bucket
	bucketLengths []int
}

func New[T Matchable](capacity int) *Fileset[T] {
	return &Fileset[T]{
		readers:       make([]T, 0, capacity),
		positions:     make([]bucketPos, 0, capacity),
		buckets:       make(map[int]map[string]*bucket),
		bucketLengths: make([]int, 0),
	}
}

func (set *Fileset[T]) Len() int {
	return len(set.readers)
}

func (set *Fileset[T]) Get() []T {
	return set.readers
}

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
	clear(set.buckets)
	set.bucketLengths = set.bucketLengths[:0]
}

func (set *Fileset[T]) Reindex() {
	if len(set.readers) == 0 {
		clear(set.buckets)
		set.bucketLengths = set.bucketLengths[:0]
		return
	}

	clear(set.buckets)
	set.bucketLengths = set.bucketLengths[:0]

	for idx, reader := range set.readers {
		pos := bucketPos{}
		if fp := reader.GetFingerprint(); fp != nil && fp.Len() > 0 {
			pos = set.insertIntoBucket(idx, fp)
		}
		set.positions[idx] = pos
	}
}

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

func (set *Fileset[T]) MatchEqual(fp *fingerprint.Fingerprint) T {
	var zero T
	if fp == nil || fp.Len() == 0 {
		return zero
	}
	bucketsByLength, ok := set.buckets[fp.Len()]
	if !ok {
		return zero
	}
	bucket := bucketsByLength[fp.Key()]
	if bucket == nil {
		return zero
	}
	for _, idx := range slices.Backward(bucket.indices) {
		if fp.Equal(set.readers[idx].GetFingerprint()) {
			return set.removeAt(idx)
		}
	}
	return zero
}

func (set *Fileset[T]) MatchStartsWith(fp *fingerprint.Fingerprint) T {
	var zero T
	if fp == nil || fp.Len() == 0 {
		return zero
	}
	fpKey := fp.Key()
	for _, length := range set.bucketLengths {
		if length > len(fpKey) {
			continue
		}
		bucketsByLength, ok := set.buckets[length]
		if !ok {
			continue
		}
		bucket := bucketsByLength[fpKey[:length]]
		if bucket == nil {
			continue
		}
		for _, idx := range slices.Backward(bucket.indices) {
			if fp.StartsWith(set.readers[idx].GetFingerprint()) {
				return set.removeAt(idx)
			}
		}
	}
	return zero
}

func (set *Fileset[T]) Match(fp *fingerprint.Fingerprint, cmp func(a, b *fingerprint.Fingerprint) bool) T {
	var val T
	for idx, r := range set.readers {
		if cmp(fp, r.GetFingerprint()) {
			return set.removeAt(idx)
		}
	}
	return val
}

func (set *Fileset[T]) insertIntoBucket(idx int, fp *fingerprint.Fingerprint) bucketPos {
	keyLen := fp.Len()
	key := fp.Key()

	bucketsByLength := set.buckets[keyLen]
	if bucketsByLength == nil {
		bucketsByLength = make(map[string]*bucket)
		set.buckets[keyLen] = bucketsByLength
		set.bucketLengths = append(set.bucketLengths, keyLen)
		slices.SortFunc(set.bucketLengths, func(a, b int) int {
			return b - a
		})
	}

	b := bucketsByLength[key]
	if b == nil {
		b = &bucket{}
		bucketsByLength[key] = b
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

// comparators
func StartsWith(a, b *fingerprint.Fingerprint) bool {
	return a.StartsWith(b)
}

func Equal(a, b *fingerprint.Fingerprint) bool {
	return a.Equal(b)
}
