// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package db // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kedascalerexporter/tsdb"

import (
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/tsdb/chunks"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
)

type sample struct {
	Timestamp      int64                     `json:"timestamp"`
	FloatValue     float64                   `json:"float_value"`
	Histogram      *histogram.Histogram      `json:"histogram"`
	FloatHistogram *histogram.FloatHistogram `json:"float_histogram"`
}

func newSample(
	t int64,
	v float64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
) chunks.Sample {
	return sample{Timestamp: t, FloatValue: v, Histogram: h, FloatHistogram: fh}
}

func (s sample) T() int64 {
	return s.Timestamp
}

func (s sample) F() float64 {
	return s.FloatValue
}

func (s sample) H() *histogram.Histogram {
	return s.Histogram
}

func (s sample) FH() *histogram.FloatHistogram {
	return s.FloatHistogram
}

func (s sample) Type() chunkenc.ValueType {
	switch {
	case s.Histogram != nil:
		return chunkenc.ValHistogram
	case s.FloatHistogram != nil:
		return chunkenc.ValFloatHistogram
	default:
		return chunkenc.ValFloat
	}
}

func (s sample) Copy() chunks.Sample {
	c := sample{Timestamp: s.Timestamp, FloatValue: s.FloatValue}
	if s.Histogram != nil {
		c.Histogram = s.Histogram.Copy()
	}
	if s.FloatHistogram != nil {
		c.FloatHistogram = s.FloatHistogram.Copy()
	}
	return c
}

type InMemorySeries struct {
	Labels  labels.Labels
	Samples []chunks.Sample
}

type InMemoryDB struct {
	series map[uint64]*InMemorySeries
	mutex  sync.RWMutex
}

func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		series: make(map[uint64]*InMemorySeries),
	}
}

func (db *InMemoryDB) GetSeries() map[uint64]*InMemorySeries {
	return db.series
}

func (db *InMemoryDB) Querier(mint, maxt int64) (storage.Querier, error) {
	return NewQuerier(mint, maxt, db), nil
}

func (db *InMemoryDB) Appender() storage.Appender {
	return NewInMemoryAppender(db)
}

func (db *InMemoryDB) Cleanup(retention time.Duration) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	cutoff := time.Now().Add(-retention).UnixMilli()
	for key, series := range db.series {

		var filteredSample []chunks.Sample
		for _, s := range series.Samples {
			if s.T() >= cutoff {
				filteredSample = append(filteredSample, s)
			}
		}
		series.Samples = filteredSample

		if len(series.Samples) == 0 {
			delete(db.series, key)
		}
	}
}
