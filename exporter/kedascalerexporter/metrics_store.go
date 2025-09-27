// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kedascalerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kedascalerexporter"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	tsdb "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kedascalerexporter/tsdb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"
)

type OTLPMetricStore struct {
	db                *tsdb.InMemoryDB
	engine            *promql.Engine
	appender          storage.Appender
	ctx               context.Context
	mutex             sync.RWMutex
	retentionDuration time.Duration
	lastCleanup       time.Time
	logger            *zap.Logger
}

func NewOTLPMetricStore(
	ctx context.Context,
	config *Config,
	logger *zap.Logger,
) (*OTLPMetricStore, error) {
	db := tsdb.NewInMemoryDB()

	// Create PromQL engine
	engineOpts := promql.EngineOpts{
		Logger:               nil,
		Reg:                  nil,
		MaxSamples:           config.EngineSettings.MaxSamples,
		Timeout:              config.EngineSettings.Timeout,
		ActiveQueryTracker:   nil,
		LookbackDelta:        config.EngineSettings.LookbackDelta,
		EnableAtModifier:     true,
		EnableNegativeOffset: true,
	}

	return &OTLPMetricStore{
		logger:            logger,
		db:                db,
		engine:            promql.NewEngine(engineOpts),
		appender:          db.Appender(),
		ctx:               ctx,
		retentionDuration: config.EngineSettings.RetentionDuration,
		lastCleanup:       time.Now(),
		mutex:             sync.RWMutex{},
	}, nil
}

func (s *OTLPMetricStore) ProcessMetrics(md pmetric.Metrics) error {
	tsMap, err := prometheusremotewrite.FromMetrics(md, prometheusremotewrite.Settings{})
	if err != nil {
		// TODO add metrics
		return fmt.Errorf("failed to convert metrics: %w", err)
	}

	b := labels.NewScratchBuilder(0)
	for _, ts := range tsMap {
		ls := ts.ToLabels(&b, nil)
		for _, sample := range ts.Samples {
			// Append the sample to the appender
			_, err := s.appender.Append(0, ls, sample.Timestamp, sample.Value)
			if err != nil {
				return fmt.Errorf("failed to append sample: %w", err)
			}
		}
		for i := range ts.Histograms {
			hp := &ts.Histograms[i]
			var appendErr error
			if hp.IsFloatHistogram() {
				_, appendErr = s.appender.AppendHistogram(0, ls, hp.Timestamp, nil, hp.ToFloatHistogram())
			} else {
				_, appendErr = s.appender.AppendHistogram(0, ls, hp.Timestamp, hp.ToIntHistogram(), nil)
			}
			if appendErr != nil {
				// Although AppendHistogram does not currently return ErrDuplicateSampleForTimestamp there is
				// a note indicating its inclusion in the future.
				if errors.Is(appendErr, storage.ErrOutOfOrderSample) ||
					errors.Is(appendErr, storage.ErrOutOfBounds) ||
					errors.Is(appendErr, storage.ErrDuplicateSampleForTimestamp) {
					s.logger.Error(
						"Out of order histogram",
						zap.Error(appendErr),
						zap.Int64("timestamp", hp.Timestamp),
					)
				}
				return appendErr
			}
		}
	}

	// Periodically clean up old data
	if time.Since(s.lastCleanup) > 30*time.Second {
		go s.CleanupExpiredData()
	}

	return nil
}

// cleanupExpiredData triggers compaction and cleanup of expired data
func (s *OTLPMetricStore) CleanupExpiredData() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.db.Cleanup(s.retentionDuration)

	s.lastCleanup = time.Now()
}

func (s *OTLPMetricStore) GetSeries() map[uint64]*tsdb.InMemorySeries {
	return s.db.GetSeries()
}

func (s *OTLPMetricStore) GetSeriesNames() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	series := s.db.GetSeries()
	namesSet := make(map[string]bool, len(series))
	for _, s := range series {
		namesSet[s.Labels.Get(labels.MetricName)] = true
	}

	names := make([]string, 0, len(namesSet))
	for name := range namesSet {
		names = append(names, name)
	}

	return names
}

func (s *OTLPMetricStore) Query(query string, ts time.Time) (float64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Parse the query
	q, err := s.engine.NewInstantQuery(s.ctx, s.db, nil, query, ts)
	if err != nil {
		return 0, fmt.Errorf("failed to create query: %w", err)
	}
	defer q.Close()

	// Execute the query
	res := q.Exec(s.ctx)
	if res.Err != nil {
		return 0, fmt.Errorf("query execution failed: %w", res.Err)
	}

	// Extract and return the result vector
	switch vector := res.Value.(type) {
	case promql.Vector:

		// If no results, return 0
		if len(vector) == 0 {
			return 0, nil
		}

		if len(vector) > 1 {
			return 0, fmt.Errorf("multiple results found for query: %s", query)
		}

		// Return the first value from the vector
		return vector[0].F, nil
	default:
		return 0, fmt.Errorf("unexpected result type: %T", res.Value)
	}
}

// Close cleans up resources used by the store
func (*OTLPMetricStore) Close() error {
	return nil
}
