// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"fmt"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
)

type InMemoryAppender struct {
	db *InMemoryDB
}

func (a *InMemoryAppender) SetOptions(*storage.AppendOptions) {
	// No options to set for in-memory appender.
}

func NewInMemoryAppender(db *InMemoryDB) storage.Appender {
	return &InMemoryAppender{
		db: db,
	}
}

func (a *InMemoryAppender) getOrCreateSeries(l labels.Labels) *InMemorySeries {
	key := l.Hash()
	series, ok := a.db.series[key]
	if !ok {
		series = &InMemorySeries{Labels: l}
		a.db.series[key] = series
		return series
	}
	return series
}

func (a *InMemoryAppender) AppendHistogram(
	_ storage.SeriesRef,
	l labels.Labels,
	t int64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
) (storage.SeriesRef, error) {
	a.db.mutex.Lock()
	defer a.db.mutex.Unlock()
	if h != nil {
		if err := h.Validate(); err != nil {
			return 0, err
		}
	}

	if fh != nil {
		if err := fh.Validate(); err != nil {
			return 0, err
		}
	}

	l = l.WithoutEmpty()
	if l.IsEmpty() {
		return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
	}

	if lbl, dup := l.HasDuplicateLabelNames(); dup {
		return 0, fmt.Errorf(`label name "%s" is not unique: %w`, lbl, tsdb.ErrInvalidSample)
	}

	series := a.getOrCreateSeries(l)

	switch {
	case h != nil:
		series.Samples = append(series.Samples, newSample(t, 0, h, nil))
	case fh != nil:
		series.Samples = append(series.Samples, newSample(t, 0, nil, fh))
	}

	return 0, nil
}

func (a *InMemoryAppender) Append(
	_ storage.SeriesRef,
	l labels.Labels,
	t int64,
	v float64,
) (storage.SeriesRef, error) {
	a.db.mutex.Lock()
	defer a.db.mutex.Unlock()

	l = l.WithoutEmpty()
	if l.IsEmpty() {
		return 0, fmt.Errorf("empty labelset: %w", tsdb.ErrInvalidSample)
	}

	if lbl, dup := l.HasDuplicateLabelNames(); dup {
		return 0, fmt.Errorf(`label name "%s" is not unique: %w`, lbl, tsdb.ErrInvalidSample)
	}

	series := a.getOrCreateSeries(l)

	series.Samples = append(series.Samples, newSample(t, v, nil, nil))
	return 0, nil
}

func (a *InMemoryAppender) UpdateMetadata(
	ref storage.SeriesRef,
	l labels.Labels,
	m metadata.Metadata,
) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *InMemoryAppender) AppendHistogramCTZeroSample(
	ref storage.SeriesRef,
	l labels.Labels,
	t, ct int64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
) (storage.SeriesRef, error) {
	return a.AppendHistogram(ref, l, t, h, fh)
}

func (a *InMemoryAppender) AppendCTZeroSample(
	ref storage.SeriesRef,
	l labels.Labels,
	t int64,
	ct int64,
) (storage.SeriesRef, error) {
	return a.Append(ref, l, t, 0)
}

func (a *InMemoryAppender) AppendExemplar(
	ref storage.SeriesRef,
	l labels.Labels,
	e exemplar.Exemplar,
) (storage.SeriesRef, error) {
	return ref, nil
}
func (a *InMemoryAppender) Commit() error   { return nil }
func (a *InMemoryAppender) Rollback() error { return nil }
