// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"

import (
	"encoding/binary"
	"path/filepath"
	"sync/atomic"

	"github.com/cockroachdb/pebble/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const (
	traceIDSeparator byte = ':'
	traceIDBytes          = len(pcommon.TraceID{})

	// storageVersion is a version to support evolution.
	storageVersion = "v0"
)

type storage struct {
	db          *pebble.DB
	logger      *zap.Logger
	nextSeq     atomic.Uint64
	unmarshaler ptrace.Unmarshaler
	marshaler   ptrace.Marshaler
}

func newStorage(storageDir string, logger *zap.Logger) (*storage, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	db, err := newPebbleDB(filepath.Join(storageDir, storageVersion), logger)
	if err != nil {
		return nil, err
	}

	return &storage{
		db:          db,
		logger:      logger,
		marshaler:   &ptrace.ProtoMarshaler{},
		unmarshaler: &ptrace.ProtoUnmarshaler{},
	}, nil
}

func (s *storage) Close() error {
	return s.db.Close()
}

func (s *storage) Append(traceID pcommon.TraceID, rss ptrace.ResourceSpans) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rss.MoveTo(rs)

	data, err := s.marshaler.MarshalTraces(td)
	if err != nil {
		s.logger.Warn("failed to marshal trace payload for tail storage", zap.Error(err))
		return
	}

	key := traceEntryKey(traceID, s.nextSeq.Add(1))
	if err := s.db.Set(key[:], data, pebble.NoSync); err != nil {
		s.logger.Warn("failed to append trace payload to tail storage", zap.Error(err))
	}
}

func (s *storage) Take(traceID pcommon.TraceID) (ptrace.Traces, bool) {
	prefix := tracePrefix(traceID)
	out := s.readByTracePrefix(prefix[:])
	if out.ResourceSpans().Len() == 0 {
		return ptrace.NewTraces(), false
	}
	end := tracePrefixUpperBound(prefix)
	if err := s.db.DeleteRange(prefix[:], end[:], pebble.NoSync); err != nil {
		s.logger.Warn("failed to delete taken trace payload range from tail storage", zap.Error(err))
	}
	return out, true
}

func (s *storage) Delete(traceID pcommon.TraceID) {
	prefix := tracePrefix(traceID)
	// Delete all entries for the trace in one range operation instead of
	// iterating keys and deleting one-by-one.
	end := tracePrefixUpperBound(prefix)
	if err := s.db.DeleteRange(prefix[:], end[:], pebble.NoSync); err != nil {
		s.logger.Warn("failed to delete trace payload range from tail storage", zap.Error(err))
	}
}

func (s *storage) readByTracePrefix(prefix []byte) ptrace.Traces {
	iter, err := s.db.NewIter(nil)
	if err != nil {
		s.logger.Warn("failed to create tail storage iterator", zap.Error(err))
		return ptrace.NewTraces()
	}
	defer iter.Close()

	// SeekPrefixGE enables prefix bloom filter usage when configured in Pebble options.
	if ok := iter.SeekPrefixGE(prefix); !ok {
		return ptrace.NewTraces()
	}

	result := ptrace.NewTraces()
	for ; iter.Valid(); iter.Next() {
		val, err := iter.ValueAndErr()
		if err != nil {
			s.logger.Warn("failed to read trace payload from tail storage", zap.Error(err))
			continue
		}

		td, err := s.unmarshaler.UnmarshalTraces(val)
		if err != nil {
			s.logger.Warn("failed to unmarshal trace payload from tail storage", zap.Error(err))
			continue
		}

		rs := td.ResourceSpans()
		for i := 0; i < rs.Len(); i++ {
			dest := result.ResourceSpans().AppendEmpty()
			rs.At(i).MoveTo(dest)
		}
	}

	if err := iter.Error(); err != nil {
		s.logger.Warn("tail storage iterator error", zap.Error(err))
	}

	return result
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
