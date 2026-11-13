// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension/internal/metadata"
)

const (
	traceIDSeparator byte = ':'
	traceIDBytes          = len(pcommon.TraceID{})

	// storageVersion is a version to support evolution.
	storageVersion = "v0"

	sizeCheckInterval = time.Second
)

type storageIter interface {
	SeekPrefixGE([]byte) bool
	Valid() bool
	Next() bool
	ValueAndErr() ([]byte, error)
	Error() error
	Close() error
}

var (
	errStorageLimitReached = errors.New("pebble tail storage size limit reached")
	errReadErrorDropTrace  = errors.New("pebble tail storage read error, trace dropped")
)

type storage struct {
	db               *pebble.DB
	logger           *zap.Logger
	telemetry        *metadata.TelemetryBuilder
	maxSize          uint64
	onReadError      ReadErrorPolicy
	nextSeq          atomic.Uint64
	lastObservedSize atomic.Uint64
	unmarshaler      ptrace.Unmarshaler
	marshaler        ptrace.Marshaler
	newIter          func() (storageIter, error)

	diskUsage            func() uint64
	stopSizeMonitor      context.CancelFunc
	sizeMonitorWaitGroup sync.WaitGroup
}

func newStorage(ctx context.Context, cfg *Config, logger *zap.Logger, telemetry *metadata.TelemetryBuilder) (*storage, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	db, created, err := newPebbleDB(filepath.Join(cfg.Directory, storageVersion), logger)
	if err != nil {
		return nil, err
	}

	s := &storage{
		db:          db,
		logger:      logger,
		telemetry:   telemetry,
		maxSize:     uint64(cfg.MaxStorageSizeMiB) << 20,
		onReadError: cfg.OnReadError,
		marshaler:   &ptrace.ProtoMarshaler{},
		unmarshaler: &ptrace.ProtoUnmarshaler{},
	}
	if s.onReadError == "" {
		s.onReadError = ReadErrorPolicyDropTrace
	}
	s.newIter = func() (storageIter, error) {
		return s.db.NewIter(nil)
	}
	s.diskUsage = func() uint64 {
		return s.db.Metrics().DiskSpaceUsage()
	}

	if !created {
		// Persistence across restarts is not supported.
		// Enforce this at startup to prevent users from relying on persistence.
		logger.Warn("existing database found; dropping all data as persistence across restarts is not supported")
		if err := s.drop(ctx); err != nil {
			return nil, err
		}
	}

	if s.maxSize > 0 {
		s.updateDiskUsage()
		monitorCtx, cancel := context.WithCancel(context.Background())
		s.stopSizeMonitor = cancel
		s.sizeMonitorWaitGroup.Go(func() {
			s.monitorDiskUsage(monitorCtx)
		})
	}

	return s, nil
}

func (s *storage) drop(ctx context.Context) error {
	var lo, hi [traceIDBytes + 1]byte
	lo[len(lo)-1] = traceIDSeparator
	for i := range hi {
		if i == len(hi)-1 {
			hi[i] = traceIDSeparator + 1 // +1 to include the greatest trace ID with trace ID separator
			break
		}
		hi[i] = 0xff
	}
	if err := s.db.DeleteRange(lo[:], hi[:], pebble.NoSync); err != nil {
		return err
	}
	if err := s.db.Compact(ctx, lo[:], hi[:], true); err != nil {
		return err
	}
	return nil
}

func (s *storage) Close() error {
	if s.stopSizeMonitor != nil {
		s.stopSizeMonitor()
		s.sizeMonitorWaitGroup.Wait()
	}
	return s.db.Close()
}

func (s *storage) Append(traceID pcommon.TraceID, td ptrace.Traces) error {
	data, err := s.marshaler.MarshalTraces(td)
	if err != nil {
		return fmt.Errorf("failed to marshal trace payload: %w", err)
	}

	seq := s.nextSeq.Add(1)
	key := traceEntryKey(traceID, seq)

	if err := s.ensureCapacity(); err != nil {
		return err
	}

	if err := s.db.Set(key[:], data, pebble.NoSync); err != nil {
		return fmt.Errorf("pebble Set error: %w", err)
	}
	return nil
}

func (s *storage) Take(traceID pcommon.TraceID) (ptrace.Traces, error) {
	prefix := tracePrefix(traceID)
	out, readFailed := s.readByTracePrefix(prefix[:])
	if !readFailed && out.ResourceSpans().Len() == 0 {
		return out, nil
	}
	end := tracePrefixUpperBound(prefix)
	if err := s.db.DeleteRange(prefix[:], end[:], pebble.NoSync); err != nil {
		return ptrace.NewTraces(), fmt.Errorf("pebble DeleteRange error: %w", err)
	}
	if !readFailed {
		return out, nil
	}
	return s.resolveReadError(traceID, out)
}

func (s *storage) Delete(traceID pcommon.TraceID) error {
	prefix := tracePrefix(traceID)
	// Delete all entries for the trace in one range operation instead of
	// iterating keys and deleting one-by-one.
	end := tracePrefixUpperBound(prefix)
	if err := s.db.DeleteRange(prefix[:], end[:], pebble.NoSync); err != nil {
		return fmt.Errorf("pebble DeleteRange error: %w", err)
	}
	return nil
}

func (s *storage) resolveReadError(traceID pcommon.TraceID, out ptrace.Traces) (ptrace.Traces, error) {
	switch s.onReadError {
	case ReadErrorPolicyReturnPartial:
		if s.telemetry != nil {
			s.telemetry.ExtensionPebbleTailStorageReadErrorPartialReturns.Add(context.Background(), 1)
		}
		s.logger.Warn("returning partial trace after tail storage read error",
			zap.Stringer("trace_id", traceID),
			zap.String("on_read_error", string(ReadErrorPolicyReturnPartial)),
			zap.Int("resource_spans", out.ResourceSpans().Len()))
		return out, nil
	default:
		if s.telemetry != nil {
			s.telemetry.ExtensionPebbleTailStorageReadErrorTraceDrops.Add(context.Background(), 1)
		}
		s.logger.Warn("dropping trace after tail storage read error",
			zap.Stringer("trace_id", traceID),
			zap.String("on_read_error", string(ReadErrorPolicyDropTrace)))
		return ptrace.NewTraces(), errReadErrorDropTrace
	}
}

// TakeMany reads and deletes several traces with one iterator and one
// write batch. Trace IDs are visited in key order so the iterator moves
// forward through the SSTables instead of seeking back and forth.
func (s *storage) TakeMany(traceIDs []pcommon.TraceID) ([]ptrace.Traces, []error) {
	n := len(traceIDs)
	out := make([]ptrace.Traces, n)
	errs := make([]error, n)
	if n == 0 {
		return out, errs
	}

	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool {
		return bytes.Compare(traceIDs[order[a]][:], traceIDs[order[b]][:]) < 0
	})

	iter, err := s.newIter()
	if err != nil {
		if s.telemetry != nil {
			s.telemetry.ExtensionPebbleTailStorageReadErrors.Add(context.Background(), 1)
		}
		s.logger.Warn("failed to create tail storage iterator", zap.Error(err))
		iter = nil
	}

	batch := s.db.NewBatch()
	defer batch.Close()

	type deletion struct {
		index      int
		result     ptrace.Traces
		readFailed bool
	}
	deletions := make([]deletion, 0, n)
	for _, i := range order {
		prefix := tracePrefix(traceIDs[i])
		result := ptrace.NewTraces()
		readFailed := true
		if iter != nil {
			result, readFailed = s.readTraceFromIter(iter, prefix[:])
			if !readFailed && result.ResourceSpans().Len() == 0 {
				out[i] = result
				continue
			}
		}
		end := tracePrefixUpperBound(prefix)
		if err := batch.DeleteRange(prefix[:], end[:], nil); err != nil {
			out[i] = ptrace.NewTraces()
			errs[i] = fmt.Errorf("pebble DeleteRange error: %w", err)
			continue
		}
		deletions = append(deletions, deletion{index: i, result: result, readFailed: readFailed})
	}

	if iter != nil {
		if err := iter.Close(); err != nil {
			s.logger.Warn("failed to close tail storage iterator", zap.Error(err))
		}
	}

	if len(deletions) == 0 {
		return out, errs
	}
	if err := batch.Commit(pebble.NoSync); err != nil {
		for _, d := range deletions {
			out[d.index] = ptrace.NewTraces()
			errs[d.index] = fmt.Errorf("pebble batch commit error: %w", err)
		}
		return out, errs
	}

	for _, d := range deletions {
		if !d.readFailed {
			out[d.index] = d.result
			continue
		}
		out[d.index], errs[d.index] = s.resolveReadError(traceIDs[d.index], d.result)
	}
	return out, errs
}

func (s *storage) readByTracePrefix(prefix []byte) (ptrace.Traces, bool) {
	iter, err := s.newIter()
	if err != nil {
		if s.telemetry != nil {
			s.telemetry.ExtensionPebbleTailStorageReadErrors.Add(context.Background(), 1)
		}
		s.logger.Warn("failed to create tail storage iterator", zap.Error(err))
		return ptrace.NewTraces(), true
	}
	defer iter.Close()

	return s.readTraceFromIter(iter, prefix)
}

func (s *storage) readTraceFromIter(iter storageIter, prefix []byte) (ptrace.Traces, bool) {
	failed := false
	result := ptrace.NewTraces()
	// SeekPrefixGE enables prefix bloom filter usage when configured in Pebble options.
	// Do not return early when the seek fails: SeekPrefixGE also returns false on
	// iterator errors, which must be surfaced by the iter.Error() check below.
	for valid := iter.SeekPrefixGE(prefix); valid; valid = iter.Next() {
		val, err := iter.ValueAndErr()
		if err != nil {
			if s.telemetry != nil {
				s.telemetry.ExtensionPebbleTailStorageReadErrors.Add(context.Background(), 1)
			}
			s.logger.Warn("failed to read trace payload from tail storage", zap.Error(err))
			failed = true
			continue
		}

		td, err := s.unmarshaler.UnmarshalTraces(val)
		if err != nil {
			if s.telemetry != nil {
				s.telemetry.ExtensionPebbleTailStorageReadErrors.Add(context.Background(), 1)
			}
			s.logger.Warn("failed to unmarshal trace payload from tail storage", zap.Error(err))
			failed = true
			continue
		}

		rs := td.ResourceSpans()
		for i := 0; i < rs.Len(); i++ {
			dest := result.ResourceSpans().AppendEmpty()
			rs.At(i).MoveTo(dest)
		}
	}

	if err := iter.Error(); err != nil {
		if s.telemetry != nil {
			s.telemetry.ExtensionPebbleTailStorageReadErrors.Add(context.Background(), 1)
		}
		s.logger.Warn("tail storage iterator error", zap.Error(err))
		failed = true
	}

	return result, failed
}

func tracePrefix(traceID pcommon.TraceID) (prefix [traceIDBytes + 1]byte) {
	copy(prefix[:], traceID[:])
	prefix[traceIDBytes] = traceIDSeparator
	return prefix
}

func tracePrefixUpperBound(prefix [traceIDBytes + 1]byte) (upper [traceIDBytes + 1]byte) {
	upper = prefix // copy
	upper[len(upper)-1]++
	return upper
}

func traceEntryKey(traceID pcommon.TraceID, seq uint64) (key [traceIDBytes + 1 + 8]byte) {
	copy(key[:], traceID[:])
	key[traceIDBytes] = traceIDSeparator
	binary.BigEndian.PutUint64(key[traceIDBytes+1:], seq)
	return key
}

func (s *storage) monitorDiskUsage(ctx context.Context) {
	ticker := time.NewTicker(sizeCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateDiskUsage()
		case <-ctx.Done():
			return
		}
	}
}

func (s *storage) updateDiskUsage() {
	s.lastObservedSize.Store(s.diskUsage())
}

func (s *storage) ensureCapacity() error {
	if s.maxSize == 0 {
		return nil
	}
	lastObservedSize := s.lastObservedSize.Load()
	if lastObservedSize > s.maxSize {
		return fmt.Errorf("%w: last observed database size %d exceeds configured limit %d", errStorageLimitReached, lastObservedSize, s.maxSize)
	}
	return nil
}
