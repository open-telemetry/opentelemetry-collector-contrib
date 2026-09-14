// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"
	eventsv1 "k8s.io/api/events/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
)

const (
	// Number of log attributes to add to the plog.LogRecordSlice.
	totalLogAttributes = 8

	// Number of resource attributes to add to the plog.ResourceLogs.
	totalResourceAttributes = 7
)

// By default k8s event has only two types of events (Normal, Warning), here are we allowing other types as well.
// For more info: https://github.com/kubernetes/api/blob/release-1.34/events/v1/types_swagger_doc_generated.go#L42
var severityMap = map[string]plog.SeverityNumber{
	"normal":   plog.SeverityNumberInfo,
	"warning":  plog.SeverityNumberWarn,
	"error":    plog.SeverityNumberError,
	"critical": plog.SeverityNumberFatal,
}

// k8sEventToLogRecord converts a Kubernetes events.k8s.io/v1 Event to plog.Logs and adds the resource attributes.
func k8sEventToLogData(logger *zap.Logger, ev *eventsv1.Event, version string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName(metadata.ScopeName)
	sl.Scope().SetVersion(version)
	lr := sl.LogRecords().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(totalResourceAttributes)

	// k8s.namespace.name describes the regarding object's namespace and belongs as a resource attribute.
	resourceAttrs.PutStr(string(conventions.K8SNamespaceNameKey), ev.Regarding.Namespace)

	// Attributes related to the object causing the event (regarding in events.k8s.io/v1).
	resourceAttrs.PutStr("k8s.object.kind", ev.Regarding.Kind)
	resourceAttrs.PutStr("k8s.object.name", ev.Regarding.Name)
	resourceAttrs.PutStr("k8s.object.uid", string(ev.Regarding.UID))
	resourceAttrs.PutStr("k8s.object.fieldpath", ev.Regarding.FieldPath)
	resourceAttrs.PutStr("k8s.object.api_version", ev.Regarding.APIVersion)
	resourceAttrs.PutStr("k8s.object.resource_version", ev.Regarding.ResourceVersion)

	lr.SetTimestamp(pcommon.NewTimestampFromTime(getEventTimestamp(ev)))

	// The Note field contains description about the event (was Message in core/v1),
	// which is best suited for the "Body" of the LogRecordSlice.
	lr.Body().SetStr(ev.Note)

	// Set the "SeverityNumber" and "SeverityText" if a known type of
	// severity is found.
	if severityNumber, ok := severityMap[strings.ToLower(ev.Type)]; ok {
		lr.SetSeverityNumber(severityNumber)
		lr.SetSeverityText(ev.Type)
	} else {
		logger.Debug("unknown severity type", zap.String("type", ev.Type))
	}

	attrs := lr.Attributes()
	attrs.EnsureCapacity(totalLogAttributes)

	attrs.PutStr("k8s.event.reason", ev.Reason)
	attrs.PutStr("k8s.event.action", ev.Action)
	attrs.PutStr("k8s.event.start_time", ev.CreationTimestamp.String())
	attrs.PutStr("k8s.event.name", ev.Name)
	attrs.PutStr("k8s.event.uid", string(ev.UID))
	attrs.PutStr("k8s.event.reporting_controller", ev.ReportingController)
	attrs.PutStr("k8s.event.reporting_instance", ev.ReportingInstance)

	// For series events, use series.count; for single events omit (no equivalent of core/v1 count).
	if ev.Series != nil && ev.Series.Count != 0 {
		attrs.PutInt("k8s.event.count", int64(ev.Series.Count))
	}

	return ld
}

// getEventTimestamp returns the most relevant timestamp for an events.k8s.io/v1 Event.
// Priority: series.lastObservedTime > eventTime. For series events this gives the most
// recent occurrence; for single events eventTime is the only available timestamp.
func getEventTimestamp(ev *eventsv1.Event) time.Time {
	if ev.Series != nil && ev.Series.LastObservedTime.Time != (time.Time{}) {
		return ev.Series.LastObservedTime.Time
	}
	return ev.EventTime.Time
}
