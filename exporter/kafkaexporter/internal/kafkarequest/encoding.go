// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkarequest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkarequest"

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
)

// encodingVersion identifies the wire format. Bump on any breaking change.
// Unmarshal returns an error for unknown versions, which causes the queue
// item to be dropped cleanly (per core persistent_queue semantics).
const encodingVersion uint64 = 1

// Encoding implements QueueBatchEncoding for *Request. Context is not
// preserved across persistence — headers are applied at convert time, so
// context loss after restart is harmless.
type Encoding struct{}

var _ exporterhelper.QueueBatchEncoding[xexporterhelper.Request] = Encoding{}

// NewEncoding returns the encoder for use in QueueBatchSettings.
func NewEncoding() Encoding { return Encoding{} }

func (Encoding) Marshal(_ context.Context, req xexporterhelper.Request) ([]byte, error) {
	kr, ok := req.(*Request)
	if !ok {
		return nil, fmt.Errorf("kafkarequest: Marshal got %T, expected *Request", req)
	}

	buf := make([]byte, 0, kr.bytesSize+len(kr.records)*16+8)
	buf = binary.AppendUvarint(buf, encodingVersion)
	buf = binary.AppendUvarint(buf, uint64(len(kr.records)))
	for _, rec := range kr.records {
		buf = appendString(buf, rec.Topic)
		buf = appendBytes(buf, rec.Key)
		buf = appendBytes(buf, rec.Value)
		buf = binary.AppendVarint(buf, int64(rec.Partition))
		buf = binary.AppendUvarint(buf, uint64(len(rec.Headers)))
		for _, h := range rec.Headers {
			buf = appendString(buf, h.Key)
			buf = appendBytes(buf, h.Value)
		}
	}
	return buf, nil
}

func (Encoding) Unmarshal(data []byte) (context.Context, xexporterhelper.Request, error) {
	d := &decoder{data: data}

	version, ok := d.readUvarint()
	if !ok {
		return context.Background(), nil, errors.New("kafkarequest: truncated header")
	}
	if version != encodingVersion {
		return context.Background(), nil, fmt.Errorf("kafkarequest: unknown encoding version %d", version)
	}

	count, ok := d.readUvarint()
	if !ok {
		return context.Background(), nil, errors.New("kafkarequest: truncated record count")
	}

	records := make([]*kgo.Record, 0, count)
	for i := range count {
		rec, err := d.readRecord()
		if err != nil {
			return context.Background(), nil, fmt.Errorf("kafkarequest: record %d: %w", i, err)
		}
		records = append(records, rec)
	}
	if !d.exhausted() {
		return context.Background(), nil, errors.New("kafkarequest: trailing bytes after records")
	}
	return context.Background(), New(records), nil
}

type decoder struct {
	data []byte
}

func (d *decoder) readUvarint() (uint64, bool) {
	v, n := binary.Uvarint(d.data)
	if n <= 0 {
		return 0, false
	}
	d.data = d.data[n:]
	return v, true
}

func (d *decoder) readVarint() (int64, bool) {
	v, n := binary.Varint(d.data)
	if n <= 0 {
		return 0, false
	}
	d.data = d.data[n:]
	return v, true
}

func (d *decoder) readBytes() ([]byte, bool) {
	length, ok := d.readUvarint()
	if !ok {
		return nil, false
	}
	if uint64(len(d.data)) < length {
		return nil, false
	}
	out := make([]byte, length)
	copy(out, d.data[:length])
	d.data = d.data[length:]
	return out, true
}

func (d *decoder) readString() (string, bool) {
	b, ok := d.readBytes()
	if !ok {
		return "", false
	}
	return string(b), true
}

func (d *decoder) readRecord() (*kgo.Record, error) {
	topic, ok := d.readString()
	if !ok {
		return nil, errors.New("truncated topic")
	}
	key, ok := d.readBytes()
	if !ok {
		return nil, errors.New("truncated key")
	}
	value, ok := d.readBytes()
	if !ok {
		return nil, errors.New("truncated value")
	}
	partition, ok := d.readVarint()
	if !ok {
		return nil, errors.New("truncated partition")
	}
	headerCount, ok := d.readUvarint()
	if !ok {
		return nil, errors.New("truncated header count")
	}
	var headers []kgo.RecordHeader
	if headerCount > 0 {
		headers = make([]kgo.RecordHeader, 0, headerCount)
		for range headerCount {
			hk, ok := d.readString()
			if !ok {
				return nil, errors.New("truncated header key")
			}
			hv, ok := d.readBytes()
			if !ok {
				return nil, errors.New("truncated header value")
			}
			headers = append(headers, kgo.RecordHeader{Key: hk, Value: hv})
		}
	}
	rec := &kgo.Record{
		Topic:     topic,
		Key:       nilIfEmpty(key),
		Value:     nilIfEmpty(value),
		Partition: int32(partition),
		Headers:   headers,
	}
	return rec, nil
}

func (d *decoder) exhausted() bool { return len(d.data) == 0 }

func appendString(buf []byte, s string) []byte {
	buf = binary.AppendUvarint(buf, uint64(len(s)))
	return append(buf, s...)
}

func appendBytes(buf, b []byte) []byte {
	buf = binary.AppendUvarint(buf, uint64(len(b)))
	return append(buf, b...)
}

func nilIfEmpty(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}
