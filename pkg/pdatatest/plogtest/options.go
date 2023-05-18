// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package plogtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"

import (
	"bytes"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// CompareLogsOption can be used to mutate expected and/or actual logs before comparing.
type CompareLogsOption interface {
	applyOnLogs(expected, actual plog.Logs)
}

type compareLogsOptionFunc func(expected, actual plog.Logs)

func (f compareLogsOptionFunc) applyOnLogs(expected, actual plog.Logs) {
	f(expected, actual)
}

// IgnoreResourceAttributeValue is a CompareLogsOption that removes a resource attribute
// from all resources.
func IgnoreResourceAttributeValue(attributeName string) CompareLogsOption {
	return ignoreResourceAttributeValue{
		attributeName: attributeName,
	}
}

type ignoreResourceAttributeValue struct {
	attributeName string
}

func (opt ignoreResourceAttributeValue) applyOnLogs(expected, actual plog.Logs) {
	opt.maskLogsResourceAttributeValue(expected)
	opt.maskLogsResourceAttributeValue(actual)
}

func (opt ignoreResourceAttributeValue) maskLogsResourceAttributeValue(metrics plog.Logs) {
	rls := metrics.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		internal.MaskResourceAttributeValue(rls.At(i).Resource(), opt.attributeName)
	}
}

func IgnoreObservedTimestamp() CompareLogsOption {
	return compareLogsOptionFunc(func(expected, actual plog.Logs) {
		now := pcommon.NewTimestampFromTime(time.Now())
		maskObservedTimestamp(expected, now)
		maskObservedTimestamp(actual, now)
	})
}

func maskObservedTimestamp(logs plog.Logs, ts pcommon.Timestamp) {
	rls := logs.ResourceLogs()
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			lrs := sls.At(j).LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lrs.At(k).SetObservedTimestamp(ts)
			}
		}
	}
}

// IgnoreResourceLogsOrder is a CompareLogsOption that ignores the order of resource traces/metrics/logs.
func IgnoreResourceLogsOrder() CompareLogsOption {
	return compareLogsOptionFunc(func(expected, actual plog.Logs) {
		sortResourceLogsSlice(expected.ResourceLogs())
		sortResourceLogsSlice(actual.ResourceLogs())
	})
}

func sortResourceLogsSlice(rls plog.ResourceLogsSlice) {
	rls.Sort(func(a, b plog.ResourceLogs) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		aAttrs := pdatautil.MapHash(a.Resource().Attributes())
		bAttrs := pdatautil.MapHash(b.Resource().Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

// IgnoreScopeLogsOrder is a CompareLogsOption that ignores the order of instrumentation scope traces/metrics/logs.
func IgnoreScopeLogsOrder() CompareLogsOption {
	return compareLogsOptionFunc(func(expected, actual plog.Logs) {
		sortScopeLogsSlices(expected)
		sortScopeLogsSlices(actual)
	})
}

func sortScopeLogsSlices(ls plog.Logs) {
	for i := 0; i < ls.ResourceLogs().Len(); i++ {
		ls.ResourceLogs().At(i).ScopeLogs().Sort(func(a, b plog.ScopeLogs) bool {
			if a.SchemaUrl() != b.SchemaUrl() {
				return a.SchemaUrl() < b.SchemaUrl()
			}
			if a.Scope().Name() != b.Scope().Name() {
				return a.Scope().Name() < b.Scope().Name()
			}
			return a.Scope().Version() < b.Scope().Version()
		})
	}
}

// IgnoreLogRecordsOrder is a CompareLogsOption that ignores the order of log records.
func IgnoreLogRecordsOrder() CompareLogsOption {
	return compareLogsOptionFunc(func(expected, actual plog.Logs) {
		sortLogRecordSlices(expected)
		sortLogRecordSlices(actual)
	})
}

func sortLogRecordSlices(ls plog.Logs) {
	for i := 0; i < ls.ResourceLogs().Len(); i++ {
		for j := 0; j < ls.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			ls.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Sort(func(a, b plog.LogRecord) bool {
				if a.ObservedTimestamp() != b.ObservedTimestamp() {
					return a.ObservedTimestamp() < b.ObservedTimestamp()
				}
				if a.Timestamp() != b.Timestamp() {
					return a.Timestamp() < b.Timestamp()
				}
				if a.SeverityNumber() != b.SeverityNumber() {
					return a.SeverityNumber() < b.SeverityNumber()
				}
				if a.SeverityText() != b.SeverityText() {
					return a.SeverityText() < b.SeverityText()
				}
				at := a.TraceID()
				bt := b.TraceID()
				if !bytes.Equal(at[:], bt[:]) {
					return bytes.Compare(at[:], bt[:]) < 0
				}
				as := a.SpanID()
				bs := b.SpanID()
				if !bytes.Equal(as[:], bs[:]) {
					return bytes.Compare(as[:], bs[:]) < 0
				}
				aAttrs := pdatautil.MapHash(a.Attributes())
				bAttrs := pdatautil.MapHash(b.Attributes())
				if !bytes.Equal(aAttrs[:], bAttrs[:]) {
					return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
				}
				ab := pdatautil.ValueHash(a.Body())
				bb := pdatautil.ValueHash(b.Body())
				return bytes.Compare(ab[:], bb[:]) < 0
			})
		}
	}
}
