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

func (set *Fileset[T]) Match(fp *fingerprint.Fingerprint, cmp func(a, b *fingerprint.Fingerprint) bool) T {
	var val T
	for idx, r := range set.readers {
		if cmp(fp, r.GetFingerprint()) {
			set.readers = append(set.readers[:idx], set.readers[idx+1:]...)
			return r
		}
	}
	return val
}

// comparators
func StartsWith(a, b *fingerprint.Fingerprint) bool {
	return a.StartsWith(b)
}

func Equal(a, b *fingerprint.Fingerprint) bool {
	return a.Equal(b)
}
