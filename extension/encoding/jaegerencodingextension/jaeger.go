// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"

import (
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type jaegerProtobufTrace struct {
}

func (j jaegerProtobufTrace) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	buff := newReadableBuffer(buf)
	slices, err := buff.slices()
	if err != nil {
		return ptrace.NewTraces(), err
	}

	batches := make([]*jaegerproto.Batch, 0)
	for _, slice := range slices {
		batch := jaegerproto.Batch{}
		e := batch.Unmarshal(slice)
		if e != nil {
			return ptrace.NewTraces(), e
		}
		batches = append(batches, &batch)
	}

	return jaeger.ProtoToTraces(batches)
}

func (j jaegerProtobufTrace) MarshalTraces(traces ptrace.Traces) ([]byte, error) {
	batches, err := jaeger.ProtoFromTraces(traces)
	if err != nil {
		return nil, err
	}

	arr := make([][]byte, 0)
	for _, batch := range batches {
		bts, e := batch.Marshal()
		if e != nil {
			return nil, e
		}
		arr = append(arr, bts)
	}

	buff, err := newWritableBuffer(calculateCap(arr))
	if err != nil {
		return nil, err
	}

	for _, bytes := range arr {
		err := buff.writeBytes(bytes)
		if err != nil {
			return nil, err
		}
	}

	return buff.bytes(), nil
}
