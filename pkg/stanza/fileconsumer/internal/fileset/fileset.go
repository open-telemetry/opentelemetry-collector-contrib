// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

type fileset struct {
	readers []*reader.Reader
}

func New(cap int) *fileset {
	return &fileset{readers: make([]*reader.Reader, 0, cap)}
}

func (set *fileset) Len() int {
	return len(set.readers)
}

func (set *fileset) Get() []*reader.Reader {
	return set.readers
}

func (set *fileset) Pop() (*reader.Reader, error) {
	// closes one file and returns metadata wrapped with a nil reader
	if len(set.readers) == 0 {
		return nil, errors.New("pop() on empty fileset")
	}
	m := set.readers[0].Close()
	set.readers = set.readers[1:]
	return &reader.Reader{Metadata: m}, nil
}

func (set *fileset) Add(readers ...*reader.Reader) {
	// add open readers
	set.readers = append(set.readers, readers...)
}

func (set *fileset) Clear() {
	// clear the underlying readers
	set.readers = make([]*reader.Reader, 0, cap(set.readers))
}

func (set *fileset) Reset(readers ...*reader.Reader) []*reader.Reader {
	// empty the underlying set and return the metadata wrapped with a nil reader
	metadata := make([]*reader.Reader, 0)
	for _, r := range set.readers {
		metadata = append(metadata, &reader.Reader{Metadata: r.Close()})
	}
	set.Clear()
	set.readers = append(set.readers, readers...)
	return metadata
}

func (set *fileset) HasMatch(fp *fingerprint.Fingerprint, cmp func(a, b *fingerprint.Fingerprint) bool) (int, bool) {
	// can be called to detect exact or partial fingerprint match
	for idx, r := range set.readers {
		if cmp(fp, r.fingerprint) {
			return idx, true
		}
	}
	return -1, false
}

func (set *fileset) RemoveMatch(fp *fingerprint.Fingerprint) *reader.Metadata {
	if idx, ok := set.HasMatch(fp, func(a, b *fingerprint.Fingerprint) bool { return a.StartsWith(b) }); ok {
		m := set.readers[idx].Close()
		set.readers = append(set.readers[:idx], set.readers[idx+1:]...)
		return m
	}
	return nil
}
