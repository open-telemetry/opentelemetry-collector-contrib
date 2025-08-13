// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Number of log attributes to add to the plog.LogRecordSlice.
	totalLogAttributes = 7

	// Number of resource attributes to add to the plog.ResourceLogs.
	totalResourceAttributes = 6
)

// Only two types of events are created as of now.
// For more info: https://docs.openshift.com/container-platform/4.9/rest_api/metadata_apis/event-core-v1.html
var severityMap = map[string]plog.SeverityNumber{
	"normal":  plog.SeverityNumberInfo,
	"warning": plog.SeverityNumberWarn,
}

// k8sEventToLogRecord converts Kubernetes event to plog.LogRecordSlice and adds the resource attributes.
func k8sEventToLogData(logger *zap.Logger, ev *corev1.Event) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(totalResourceAttributes)

	resourceAttrs.PutStr(string(semconv.K8SNodeNameKey), ev.Source.Host)

	// Attributes related to the object causing the event.
	resourceAttrs.PutStr("k8s.object.kind", ev.InvolvedObject.Kind)
	resourceAttrs.PutStr("k8s.object.name", ev.InvolvedObject.Name)
	resourceAttrs.PutStr("k8s.object.uid", string(ev.InvolvedObject.UID))
	resourceAttrs.PutStr("k8s.object.fieldpath", ev.InvolvedObject.FieldPath)
	resourceAttrs.PutStr("k8s.object.api_version", ev.InvolvedObject.APIVersion)
	resourceAttrs.PutStr("k8s.object.resource_version", ev.InvolvedObject.ResourceVersion)

	lr.SetTimestamp(pcommon.NewTimestampFromTime(getEventTimestamp(ev)))

	// The Message field contains description about the event,
	// which is best suited for the "Body" of the LogRecordSlice.
	lr.Body().SetStr(ev.Message)

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
	attrs.PutStr(string(semconv.K8SNamespaceNameKey), ev.InvolvedObject.Namespace)

	// "Count" field of k8s event will be '0' in case it is
	// not present in the collected event from k8s.
	if ev.Count != 0 {
		attrs.PutInt("k8s.event.count", int64(ev.Count))
	}

	return ld
}
