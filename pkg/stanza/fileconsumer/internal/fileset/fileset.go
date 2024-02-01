// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileset // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileset"

import (
	"errors"

	"golang.org/x/exp/slices"

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

type Fileset[T Matchable] struct {
	readers []T
}

func New[T Matchable](capacity int) *Fileset[T] {
	return &Fileset[T]{readers: make([]T, 0, capacity)}
}

func (set *Fileset[T]) Len() int {
	return len(set.readers)
}

func (set *Fileset[T]) Get() []T {
	return set.readers
}

func (set *Fileset[T]) Pop() (T, error) {
	// return first element from the array and remove it
	var val T
	if len(set.readers) == 0 {
		return val, errFilesetEmpty
	}
	r := set.readers[0]
	set.readers = slices.Delete(set.readers, 0, 1)
	return r, nil
}

func (set *Fileset[T]) Add(readers ...T) {
	// add open readers
	set.readers = append(set.readers, readers...)
}

func (set *Fileset[T]) Clear() {
	// clear the underlying readers
	set.readers = make([]T, 0, cap(set.readers))
}

func (set *Fileset[T]) Reset(readers ...T) []T {
	// empty the underlying set and return the old array
	arr := make([]T, len(set.readers))
	copy(arr, set.readers)
	set.Clear()
	set.readers = append(set.readers, readers...)
	return arr
}

func (set *Fileset[T]) Match(fp *fingerprint.Fingerprint) T {
	var val T
	for idx, r := range set.readers {
		if fp.StartsWith(r.GetFingerprint()) {
			set.readers = append(set.readers[:idx], set.readers[idx+1:]...)
			return r
		}
	}
	return val
}
