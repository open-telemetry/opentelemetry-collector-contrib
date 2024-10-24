// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"encoding/json"
	"os"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
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

	//Filled below in opsramp resource attributes
	//resourceAttrs.PutStr(semconv.AttributeK8SNodeName, ev.Source.Host)

	// Attributes related to the object causing the event.
	resourceAttrs.PutStr("k8s.object.kind", ev.InvolvedObject.Kind)
	resourceAttrs.PutStr("k8s.object.name", ev.InvolvedObject.Name)
	resourceAttrs.PutStr("k8s.object.uid", string(ev.InvolvedObject.UID))
	resourceAttrs.PutStr("k8s.object.fieldpath", ev.InvolvedObject.FieldPath)
	resourceAttrs.PutStr("k8s.object.api_version", ev.InvolvedObject.APIVersion)
	resourceAttrs.PutStr("k8s.object.resource_version", ev.InvolvedObject.ResourceVersion)

	addOpsrampAttributes(resourceAttrs, ev)

	lr.SetTimestamp(pcommon.NewTimestampFromTime(getEventTimestamp(ev)))

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
	attrs.PutStr("k8s.event.start_time", ev.ObjectMeta.CreationTimestamp.String())
	attrs.PutStr("k8s.event.name", ev.ObjectMeta.Name)
	attrs.PutStr("k8s.event.uid", string(ev.ObjectMeta.UID))
	attrs.PutStr(semconv.AttributeK8SNamespaceName, ev.InvolvedObject.Namespace)

	// "Count" field of k8s event will be '0' in case it is
	// not present in the collected event from k8s.
	if ev.Count != 0 {
		attrs.PutInt("k8s.event.count", int64(ev.Count))
	}

	attrs.PutStr("message", ev.Message)

	// The Message field contains description about the event,
	// which is best suited for the "Body" of the LogRecordSlice.

	resourceAttrs.Range(func(k string, v pcommon.Value) bool {
		attrs.PutStr(k, v.Str())
		return true
	})

	bodyMap := map[string]string{}
	attrs.Range(func(k string, v pcommon.Value) bool {
		bodyMap[k] = v.Str()
		return true
	})

	body, err := json.Marshal(bodyMap)
	if err != nil {
		logger.Debug("failed to marshal")
	}

	lr.Body().SetStr(string(body))

	return ld
}

func addOpsrampAttributes(resourceAttrs pcommon.Map, ev *corev1.Event) {
	resourceAttrs.PutStr("source", "kubernetes")
	resourceAttrs.PutStr("type", "event")
	resourceAttrs.PutStr("level", "Unknown")

	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName != "" {
		resourceAttrs.PutStr("cluster_name", clusterName) // TBD to deprecate
		resourceAttrs.PutStr("k8s.cluster.name", clusterName)
	}

	host := ""
	if ev.ReportingInstance != "" {
		host = ev.ReportingInstance
	} else if ev.Source.Host != "" {
		host = ev.Source.Host
	} else {
		host = clusterName
	}

	resourceAttrs.PutStr(semconv.AttributeK8SNodeName, host)
	resourceAttrs.PutStr("host", host)

	clusterUuid := os.Getenv("CLUSTER_UUID")
	resourceAttrs.PutStr("resourceUUID", clusterUuid)

	resourceAttrs.PutStr("namespace", ev.InvolvedObject.Namespace) // TBD to deprecate
	resourceAttrs.PutStr(semconv.AttributeK8SNamespaceName, ev.InvolvedObject.Namespace)

	if ev.InvolvedObject.Kind == "Pod" {
		resourceAttrs.PutStr("k8s.pod.uid", string(ev.InvolvedObject.UID))
	}

	if ev.InvolvedObject.Kind == "Node" {
		resourceAttrs.PutStr("k8s.node.uid", string(ev.InvolvedObject.UID))
	}
}
