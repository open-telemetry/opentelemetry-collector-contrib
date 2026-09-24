// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestHeaderCarrierGet(t *testing.T) {
	carrier := HeaderCarrier{
		{Key: "traceparent", Value: []byte("00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01")},
		{Key: "tracestate", Value: []byte("vendor=value")},
	}
	assert.Equal(t, "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01", carrier.Get("traceparent"))
	assert.Equal(t, "vendor=value", carrier.Get("tracestate"))
	assert.Empty(t, carrier.Get("missing"))
}

func TestHeaderCarrierSet(t *testing.T) {
	var carrier HeaderCarrier
	carrier.Set("traceparent", "traceparent-value")
	carrier.Set("tracestate", "vendor=value")
	assert.Equal(t, HeaderCarrier{
		{Key: "traceparent", Value: []byte("traceparent-value")},
		{Key: "tracestate", Value: []byte("vendor=value")},
	}, carrier)

	// Setting an existing key overwrites its value in place.
	carrier.Set("traceparent", "updated-value")
	assert.Equal(t, HeaderCarrier{
		{Key: "traceparent", Value: []byte("updated-value")},
		{Key: "tracestate", Value: []byte("vendor=value")},
	}, carrier)
}

func TestHeaderCarrierKeys(t *testing.T) {
	var carrier HeaderCarrier
	assert.Empty(t, carrier.Keys())

	carrier = HeaderCarrier{
		{Key: "traceparent", Value: []byte("traceparent-value")},
		{Key: "tracestate", Value: []byte("vendor=value")},
	}
	assert.Equal(t, []string{"traceparent", "tracestate"}, carrier.Keys())
}

func TestHeaderCarrierRecordHeaders(t *testing.T) {
	record := &kgo.Record{Headers: []kgo.RecordHeader{{Key: "key", Value: []byte("value")}}}
	carrier := (*HeaderCarrier)(&record.Headers)
	carrier.Set("traceparent", "traceparent-value")
	assert.Equal(t, []kgo.RecordHeader{
		{Key: "key", Value: []byte("value")},
		{Key: "traceparent", Value: []byte("traceparent-value")},
	}, record.Headers)
}
