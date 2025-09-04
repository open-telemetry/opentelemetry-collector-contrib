package db

import (
	"context"
	"sort"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/annotations"
)

type InMemoryQuerier struct {
	mint, maxt int64
	db         *InMemoryDB
}

func NewQuerier(mint, maxt int64, db *InMemoryDB) storage.Querier {
	return &InMemoryQuerier{
		mint: mint,
		maxt: maxt,
		db:   db,
	}
}

type inMemorySeriesSet struct {
	series []storage.Series
	cur    int
}

func (s *inMemorySeriesSet) Next() bool {
	s.cur++
	return s.cur-1 < len(s.series)
}

func (s *inMemorySeriesSet) At() storage.Series {
	return s.series[s.cur-1]
}

func (s *inMemorySeriesSet) Err() error {
	return nil
}

func (s *inMemorySeriesSet) Warnings() annotations.Annotations {
	return nil
}

func matchLabels(lbls labels.Labels, matchers []*labels.Matcher) bool {
	matches := true
	for _, m := range matchers {
		value := lbls.Get(m.Name)
		if value == "" {
			matches = false
			break
		}
		if !m.Matches(value) {
			matches = false
			break
		}
	}

	return matches
}

func (q *InMemoryQuerier) Select(
	_ context.Context,
	_ bool,
	_ *storage.SelectHints,
	matchers ...*labels.Matcher,
) storage.SeriesSet {
	q.db.mutex.RLock()
	defer q.db.mutex.RUnlock()

	var result []storage.Series
	for _, s := range q.db.series {
		if !matchLabels(s.Labels, matchers) {
			continue
		}
		if len(s.Samples) > 0 {
			result = append(result, storage.NewListSeries(s.Labels, s.Samples))
		}
	}
	return &inMemorySeriesSet{series: result}
}

func (q *InMemoryQuerier) LabelNames(
	_ context.Context,
	hints *storage.LabelHints,
	matchers ...*labels.Matcher,
) ([]string, annotations.Annotations, error) {
	q.db.mutex.RLock()
	defer q.db.mutex.RUnlock()

	nameSet := make(map[string]struct{})
	for _, s := range q.db.series {
		if !matchLabels(s.Labels, matchers) {
			continue
		}
		s.Labels.Range(func(l labels.Label) {
			nameSet[l.Name] = struct{}{}
		})
	}
	var names []string
	for n := range nameSet {
		if hints != nil && hints.Limit > 0 && len(names) > hints.Limit {
			break
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil, nil
}

func (q *InMemoryQuerier) LabelValues(
	_ context.Context,
	name string,
	hints *storage.LabelHints,
	matchers ...*labels.Matcher,
) ([]string, annotations.Annotations, error) {
	q.db.mutex.RLock()
	defer q.db.mutex.RUnlock()
	valueSet := make(map[string]struct{})
	for _, s := range q.db.series {
		if !matchLabels(s.Labels, matchers) {
			continue
		}
		v := s.Labels.Get(name)
		if v != "" {
			valueSet[v] = struct{}{}
		}
	}
	var values []string
	for v := range valueSet {
		if hints != nil && hints.Limit > 0 && len(values) > hints.Limit {
			break
		}
		values = append(values, v)
	}
	sort.Strings(values)
	return values, nil, nil
}

func (q *InMemoryQuerier) Close() error {
	return nil
}
