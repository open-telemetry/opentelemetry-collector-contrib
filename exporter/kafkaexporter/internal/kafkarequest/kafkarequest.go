// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package kafkarequest provides a custom exporterhelper.Request implementation
// that carries pre-marshaled Kafka records with a precomputed byte size.
// Tracking: opentelemetry-collector-contrib#48090
package kafkarequest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkarequest"

import (
	"context"
	"errors"
	"sort"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
)

var (
	_ xexporterhelper.Request             = (*Request)(nil)
	_ xexporterhelper.RequestErrorHandler = (*Request)(nil)
)

// Request holds pre-marshaled Kafka records plus their total byte size.
// Holding records (not pdata) lets BytesSize be a field read and lets
// MergeSplit operate on a slice without re-marshaling.
type Request struct {
	records   []*kgo.Record
	bytesSize int
}

// New constructs a Request. The caller must not mutate records afterwards.
func New(records []*kgo.Record) *Request {
	return &Request{records: records, bytesSize: sumRecordSize(records)}
}

// Records returns the underlying records. Callers must not mutate.
func (r *Request) Records() []*kgo.Record { return r.records }

// ItemsCount returns the number of Kafka records (not OTLP items).
func (r *Request) ItemsCount() int { return len(r.records) }

// BytesSize returns the precomputed total record size.
func (r *Request) BytesSize() int { return r.bytesSize }

// MergeSplit merges r with req (if non-nil) and splits the result so each
// returned Request fits maxSize per the given sizer type. The last returned
// Request is guaranteed to be the smallest, per the interface contract.
func (r *Request) MergeSplit(
	_ context.Context,
	maxSize int,
	sizerType exporterhelper.RequestSizerType,
	req xexporterhelper.Request,
) ([]xexporterhelper.Request, error) {
	merged := r.records
	if req != nil {
		other, ok := req.(*Request)
		if !ok {
			return nil, errors.New("kafkarequest: MergeSplit got incompatible Request type")
		}
		merged = append(merged, other.records...)
	}

	if maxSize == 0 || len(merged) == 0 {
		return []xexporterhelper.Request{New(merged)}, nil
	}

	switch sizerType {
	case exporterhelper.RequestSizerTypeBytes:
		return splitByBytes(merged, maxSize), nil
	case exporterhelper.RequestSizerTypeItems:
		return splitByCount(merged, maxSize), nil
	case exporterhelper.RequestSizerTypeRequests:
		return []xexporterhelper.Request{New(merged)}, nil
	default:
		return nil, errors.New("kafkarequest: unsupported sizer type")
	}
}

// OnError implements RequestErrorHandler. PoC stub: returns r unchanged.
// Per-record retry requires surfacing kgo per-record errors from the producer;
// tracked as follow-up.
func (r *Request) OnError(_ error) xexporterhelper.Request { return r }

func sumRecordSize(records []*kgo.Record) int {
	var total int
	for _, rec := range records {
		total += recordSize(rec)
	}
	return total
}

func recordSize(rec *kgo.Record) int {
	s := len(rec.Key) + len(rec.Value)
	for _, h := range rec.Headers {
		s += len(h.Key) + len(h.Value)
	}
	return s
}

// splitByBytes places records into bins of at most maxSize bytes. Records
// that individually exceed maxSize are emitted as their own single-record
// Requests; the broker surfaces MessageTooLarge as a permanent failure.
// Output is sorted by size descending so the last Request is the smallest,
// per the interface contract.
func splitByBytes(records []*kgo.Record, maxSize int) []xexporterhelper.Request {
	var out []xexporterhelper.Request
	var curSize int
	start := 0
	for i, rec := range records {
		size := recordSize(rec)
		if size > maxSize {
			if i > start {
				out = append(out, New(records[start:i]))
			}
			out = append(out, New(records[i:i+1]))
			start = i + 1
			curSize = 0
			continue
		}
		if curSize+size > maxSize && i > start {
			out = append(out, New(records[start:i]))
			start = i
			curSize = 0
		}
		curSize += size
	}
	if start < len(records) {
		out = append(out, New(records[start:]))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].(*Request).BytesSize() > out[j].(*Request).BytesSize()
	})
	return out
}

func splitByCount(records []*kgo.Record, maxCount int) []xexporterhelper.Request {
	if maxCount <= 0 {
		return []xexporterhelper.Request{New(records)}
	}
	out := make([]xexporterhelper.Request, 0, (len(records)+maxCount-1)/maxCount)
	for i := 0; i < len(records); i += maxCount {
		end := min(i+maxCount, len(records))
		out = append(out, New(records[i:end]))
	}
	return out
}
