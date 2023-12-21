// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracker

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

type Fileset struct {
	readers []*reader.Reader
}

func newFileset(cap int) *Fileset {
	return &Fileset{readers: make([]*reader.Reader, 0, cap)}
}

func (set *Fileset) Len() int {
	return len(set.readers)
}

func (set *Fileset) Pop() (*reader.Reader, error) {
	// closes one file and returns metadata around a nil reader
	if len(set.readers) == 0 {
		return nil, errors.New("pop() on empty fileset")
	}
	m := set.readers[0].Close()
	set.readers = set.readers[1:]
	return &reader.Reader{Metadata: m}, nil
}

func (set *Fileset) Add(readers ...*reader.Reader) {
	// add open readers
	set.readers = append(set.readers, readers...)
}

func (set *Fileset) Clear() {
	// add open readers
	set.readers = make([]*reader.Reader, 0, cap(set.readers))
}

func (set *Fileset) Reset(readers ...*reader.Reader) []*reader.Reader {
	// empty the underlying set and return the metadata wrapped around a nil reader
	metadata := make([]*reader.Reader, 0)
	for _, r := range set.readers {
		metadata = append(metadata, &reader.Reader{Metadata: r.Close()})
	}
	set.Clear()
	set.readers = append(set.readers, readers...)
	return metadata
}

func (set *Fileset) HasExactMatch(fp *fingerprint.Fingerprint) bool {
	for _, r := range set.readers {
		if fp.Equal(r.Fingerprint) {
			return true
		}
	}
	return false
}

func (set *Fileset) HasPrefix(fp *fingerprint.Fingerprint) *reader.Metadata {
	for i, r := range set.readers {
		if fp.StartsWith(r.Fingerprint) {
			m := set.readers[i].Close()
			set.readers = append(set.readers[:i], set.readers[i+1:]...)
			return m
		}
	}
	return nil
}

func (set *Fileset) RemoveOld(i int) {
	set.readers = set.readers[i:]
}
