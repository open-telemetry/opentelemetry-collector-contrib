// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"errors"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
)

func TestPcommonMapToKeyValues(t *testing.T) {
	t.Parallel()

	attrs := pcommon.NewMap()
	attrs.PutStr("error_msg", "not enough permissions")
	attrs.PutInt("count", 3)
	attrs.PutBool("retry", true)

	nested := attrs.PutEmptyMap("details")
	nested.PutStr("component", "memory_limiter")

	list := attrs.PutEmptySlice("scrapers")
	list.AppendEmpty().SetStr("cpu")
	list.AppendEmpty().SetStr("memory")

	assert.Equal(t, []*protobufs.KeyValue{
		{
			Key: "count",
			Value: &protobufs.AnyValue{
				Value: &protobufs.AnyValue_IntValue{IntValue: 3},
			},
		},
		{
			Key: "details",
			Value: &protobufs.AnyValue{
				Value: &protobufs.AnyValue_KvlistValue{
					KvlistValue: &protobufs.KeyValueList{
						Values: []*protobufs.KeyValue{
							{
								Key: "component",
								Value: &protobufs.AnyValue{
									Value: &protobufs.AnyValue_StringValue{StringValue: "memory_limiter"},
								},
							},
						},
					},
				},
			},
		},
		{
			Key: "error_msg",
			Value: &protobufs.AnyValue{
				Value: &protobufs.AnyValue_StringValue{StringValue: "not enough permissions"},
			},
		},
		{
			Key: "retry",
			Value: &protobufs.AnyValue{
				Value: &protobufs.AnyValue_BoolValue{BoolValue: true},
			},
		},
		{
			Key: "scrapers",
			Value: &protobufs.AnyValue{
				Value: &protobufs.AnyValue_ArrayValue{
					ArrayValue: &protobufs.ArrayValue{
						Values: []*protobufs.AnyValue{
							{Value: &protobufs.AnyValue_StringValue{StringValue: "cpu"}},
							{Value: &protobufs.AnyValue_StringValue{StringValue: "memory"}},
						},
					},
				},
			},
		},
	}, pcommonMapToKeyValues(attrs))
}

func TestConvertComponentHealthAttributes(t *testing.T) {
	t.Parallel()

	attrs := pcommon.NewMap()
	attrs.PutStr("error_msg", "not enough permissions")

	statusUpdate := &status.AggregateStatus{
		Event: &mockStatusEvent{
			status:     componentstatus.StatusPermanentError,
			err:        errors.New("component failed"),
			timestamp:  time.Unix(0, 0).UTC(),
			attributes: &attrs,
		},
	}

	componentHealth := convertComponentHealth(statusUpdate)

	assert.False(t, componentHealth.Healthy)
	assert.Equal(t, "StatusPermanentError", componentHealth.Status)
	assert.Equal(t, "component failed", componentHealth.LastError)
	assert.Equal(t, []*protobufs.KeyValue{
		{
			Key: "error_msg",
			Value: &protobufs.AnyValue{
				Value: &protobufs.AnyValue_StringValue{StringValue: "not enough permissions"},
			},
		},
	}, componentHealth.Attributes)
}
