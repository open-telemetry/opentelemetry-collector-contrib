// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/stretchr/testify/assert"
)

func TestParseW3CTraceContext(t *testing.T) {
	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	traceID, spanID, err := parseW3CTraceContext(traceparent)
	assert.NoError(t, err)
	assert.Equal(t, traceID, pcommon.TraceID([16]byte{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36}))
	assert.Equal(t, spanID, pcommon.SpanID([8]byte{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7}))
}

func TestSetAttributes(t *testing.T) {
	payload := map[string]any{
		"action.id":          []any{"123", "456"},
		"view.loading_time":  123,
		"view.in_foreground": true,
		"context":            map[string]any{"browser.name": "chrome"},
	}
	attributes := pcommon.NewMap()
	setAttributes(payload, attributes)

	actionID, _ := attributes.Get("datadog.action.id")
	assert.Equal(t, []any{"123", "456"}, actionID.Slice().AsRaw())

	viewLoadingTime, _ := attributes.Get("datadog.view.loading_time")
	assert.Equal(t, 123, int(viewLoadingTime.AsRaw().(int64)))

	viewInForeground, _ := attributes.Get("datadog.view.in_foreground")
	assert.Equal(t, true, viewInForeground.AsRaw().(bool))

	context, _ := attributes.Get("datadog.context")
	assert.Equal(t, "chrome", context.Map().AsRaw()["browser.name"])
}
