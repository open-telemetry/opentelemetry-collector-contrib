// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/osquery/osquery-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

const (
	defaultClientConnectRetries = 3
	defaultReconnectTimeout     = time.Millisecond * 200
	defaultQueryTimeout         = 30 * time.Second
)

type client interface {
	Close()
	QueryRowsContext(ctx context.Context, query string) ([]map[string]string, error)
}

var _ client = &osquery.ExtensionManagerClient{}

func makeOsQueryClient(socket string) (client, error) {
	client, err := osquery.NewClient(socket, defaultQueryTimeout)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type osQueryReceiver struct {
	id           component.ID
	config       *Config
	logger       *zap.Logger
	createClient func(socket string) (client, error)
	collections  []collection.Collection
	state        *collectionState
}

func newOsQueryReceiver(cfg *Config, set receiver.Settings) *osQueryReceiver {
	return &osQueryReceiver{
		id:           set.ID,
		config:       cfg,
		logger:       set.Logger,
		createClient: makeOsQueryClient,
		collections:  resolveCollections(cfg.Collections, set.Logger),
	}
}

// start resolves the storage client used to persist collection state across
// restarts. Must be called before collect/snapshotCollect for diffing to use
// any persisted state; if never called (as in tests that build osQueryReceiver
// directly), or.state stays nil and diffing simply starts cold every cycle.
func (or *osQueryReceiver) start(ctx context.Context, host component.Host) error {
	client, err := getStorageClient(ctx, host, or.config.StorageID, or.id)
	if err != nil {
		return err
	}
	or.state = newCollectionState(client, or.logger)
	return nil
}

func (or *osQueryReceiver) shutdown(ctx context.Context) error {
	return or.state.close(ctx)
}

// resolveCollections converts the configured collection names into their Collection
// implementations, logging and skipping any name that fails to resolve.
func resolveCollections(names []string, logger *zap.Logger) []collection.Collection {
	resolved := make([]collection.Collection, 0, len(names))
	for _, name := range names {
		c, err := collection.New(name)
		if err != nil {
			logger.Warn("Skipping unknown collection", zap.String("collection", name), zap.Error(err))
			continue
		}
		resolved = append(resolved, c)
	}
	return resolved
}

func (or *osQueryReceiver) connect(retries int) (client, error) {
	c, err := or.createClient(or.config.ExtensionsSocket)
	for err != nil && retries > 0 {
		or.logger.Error("Could not connect to osquery socket, retrying", zap.Error(err))
		time.Sleep(defaultReconnectTimeout)
		c, err = or.createClient(or.config.ExtensionsSocket)
		retries--
	}

	return c, err
}

// newScopeLogs appends and returns a new ScopeLogs under a new ResourceLogs in
// ld, tagged with this receiver's scope name.
func newScopeLogs(ld plog.Logs) plog.ScopeLogs {
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutStr(string(semconv.OTelScopeNameKey), metadata.Type.String())
	return sl
}

// appendLogRecords appends one log record per row to lrs, giving raw queries,
// change-only collections, and snapshot collections the same log record shape.
func appendLogRecords(lrs plog.LogRecordSlice, query, collectionName string, rows []map[string]string, now pcommon.Timestamp) {
	for _, row := range rows {
		lr := lrs.AppendEmpty()
		lr.SetTimestamp(now)
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.SetSeverityText("INFO")
		lr.Body().SetStr(query)
		if collectionName != "" {
			lr.Attributes().PutStr("collection", collectionName)
		}
		for k, v := range row {
			lr.Attributes().PutStr(k, v)
		}
	}
}

func (or *osQueryReceiver) runQuery(ctx context.Context, ld plog.Logs, query, collectionName string) error {
	now := pcommon.NewTimestampFromTime(time.Now())

	or.logger.Debug("Running query", zap.String("query", query))

	// Use a separate connection for queries in order to be able to recover from timed out queries
	queryClient, err := or.connect(defaultClientConnectRetries)
	if err != nil {
		or.logger.Error("Could not connect to osquery socket", zap.Error(err))
		return err
	}
	defer queryClient.Close()

	rows, err := queryClient.QueryRowsContext(ctx, query)
	if err != nil {
		or.logger.Error("Error running query", zap.Error(err))
	}
	appendLogRecords(newScopeLogs(ld).LogRecords(), query, collectionName, rows, now)
	return nil
}

// collectCollection runs one collection's query and returns a plog.Logs
// containing only that collection's rows. If emitAll is false, only rows that
// are new or modified since the last run (per or.state) are emitted; either
// way, the full fresh row set is saved as the new state. A query error is
// returned without emitting anything or touching state, so a transient
// failure can't look like every previously-seen row got deleted.
func (or *osQueryReceiver) collectCollection(ctx context.Context, c collection.Collection, emitAll bool) (plog.Logs, error) {
	ld := plog.NewLogs()
	name := c.GetName()
	query := c.GetQuery()
	now := pcommon.NewTimestampFromTime(time.Now())

	or.logger.Debug("Running query", zap.String("query", query), zap.String("collection", name))

	queryClient, err := or.connect(defaultClientConnectRetries)
	if err != nil {
		or.logger.Error("Could not connect to osquery socket", zap.String("collection", name), zap.Error(err))
		return ld, err
	}
	defer queryClient.Close()

	rows, err := queryClient.QueryRowsContext(ctx, query)
	if err != nil {
		or.logger.Error("Error running query", zap.String("collection", name), zap.Error(err))
		return ld, err
	}

	toEmit := rows
	if !emitAll {
		previous, loadErr := or.state.load(ctx, name)
		if loadErr != nil {
			or.logger.Warn("Failed to load previous collection state", zap.String("collection", name), zap.Error(loadErr))
		}
		toEmit = diffRows(c.RowKey, previous, rows)
	}

	if len(toEmit) > 0 {
		appendLogRecords(newScopeLogs(ld).LogRecords(), query, name, toEmit, now)
	}
	or.state.save(ctx, name, rows)

	return ld, nil
}

type collectionResult struct {
	logs plog.Logs
	err  error
}

// collectCollectionsParallel runs all configured collections concurrently,
// one goroutine per collection, and returns each one's result. Each goroutine
// writes only to its own index, so no locking is needed around results.
func (or *osQueryReceiver) collectCollectionsParallel(ctx context.Context, emitAll bool) []collectionResult {
	results := make([]collectionResult, len(or.collections))
	var wg sync.WaitGroup
	for i, c := range or.collections {
		wg.Add(1)
		go func(i int, c collection.Collection) {
			defer wg.Done()
			logs, err := or.collectCollection(ctx, c, emitAll)
			results[i] = collectionResult{logs: logs, err: err}
		}(i, c)
	}
	wg.Wait()
	return results
}

func (or *osQueryReceiver) collect(ctx context.Context) (plog.Logs, error) {
	ld := plog.NewLogs()
	var errs []error
	for _, query := range or.config.Queries {
		errs = append(errs, or.runQuery(ctx, ld, query, ""))
	}
	for _, res := range or.collectCollectionsParallel(ctx, false) {
		res.logs.ResourceLogs().MoveAndAppendTo(ld.ResourceLogs())
		errs = append(errs, res.err)
	}
	return ld, errors.Join(errs...)
}

// snapshotCollect runs all configured collections and emits every row
// unconditionally, regardless of prior state, while still refreshing that
// state so the next change-only cycle diffs against current data.
func (or *osQueryReceiver) snapshotCollect(ctx context.Context) (plog.Logs, error) {
	ld := plog.NewLogs()
	var errs []error
	for _, res := range or.collectCollectionsParallel(ctx, true) {
		res.logs.ResourceLogs().MoveAndAppendTo(ld.ResourceLogs())
		errs = append(errs, res.err)
	}
	return ld, errors.Join(errs...)
}
