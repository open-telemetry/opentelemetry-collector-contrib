// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver

import (
	"fmt"
	"sort"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestSignalFxV2EventsToLogData(t *testing.T) {
	now := time.Now()
	msec := now.UnixNano() / 1e6

	// Put it on stack or heap so we can take a ref to it.
	userDefinedCat := sfxpb.EventCategory_USER_DEFINED

	buildDefaultSFxEvent := func() *sfxpb.Event {
		return &sfxpb.Event{
			EventType:  "shutdown",
			Timestamp:  msec,
			Category:   &userDefinedCat,
			Dimensions: buildNDimensions(3),
			Properties: mapToEventProps(map[string]any{
				"env":      "prod",
				"isActive": true,
				"rack":     5,
				"temp":     40.5,
				"nullProp": nil,
			}),
		}
	}

	buildDefaultLogs := func() plog.ScopeLogs {
		sl := plog.NewScopeLogs()
		l := sl.LogRecords().AppendEmpty()
		l.SetTimestamp(pcommon.NewTimestampFromTime(now.Truncate(time.Millisecond)))
		attrs := l.Attributes()
		attrs.PutStr("com.splunk.signalfx.event_type", "shutdown")
		attrs.PutStr("k0", "v0")
		attrs.PutStr("k1", "v1")
		attrs.PutStr("k2", "v2")
		attrs.PutInt("com.splunk.signalfx.event_category", int64(sfxpb.EventCategory_USER_DEFINED))

		propMap := attrs.PutEmptyMap("com.splunk.signalfx.event_properties")
		propMap.PutStr("env", "prod")
		propMap.PutBool("isActive", true)
		propMap.PutInt("rack", 5)
		propMap.PutDouble("temp", 40.5)
		propMap.PutEmpty("nullProp")

		return sl
	}

	tests := []struct {
		name      string
		sfxEvents []*sfxpb.Event
		expected  plog.ScopeLogs
	}{
		{
			name:      "default",
			sfxEvents: []*sfxpb.Event{buildDefaultSFxEvent()},
			expected:  buildDefaultLogs(),
		},
		{
			name: "missing category",
			sfxEvents: func() []*sfxpb.Event {
				e := buildDefaultSFxEvent()
				e.Category = nil
				return []*sfxpb.Event{e}
			}(),
			expected: func() plog.ScopeLogs {
				lrs := buildDefaultLogs()
				lrs.LogRecords().At(0).Attributes().PutEmpty("com.splunk.signalfx.event_category")
				return lrs
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := plog.NewScopeLogs()
			signalFxV2EventsToLogRecords(tt.sfxEvents, sl.LogRecords())
			assert.NoError(t, plogtest.CompareScopeLogs(tt.expected, sl))
		})
	}
}

func mapToEventProps(m map[string]any) []*sfxpb.Property {
	out := make([]*sfxpb.Property, 0, len(m))
	for k, v := range m {
		var pval sfxpb.PropertyValue

		switch t := v.(type) {
		case nil:
		case string:
			pval.StrValue = &t
		case int:
			asInt := int64(t)
			pval.IntValue = &asInt
		case int64:
			pval.IntValue = &t
		case bool:
			pval.BoolValue = &t
		case float64:
			pval.DoubleValue = &t
		default:
			panic(fmt.Sprintf("invalid type: %v", v))
		}

		out = append(out, &sfxpb.Property{
			Key:   k,
			Value: &pval,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}
