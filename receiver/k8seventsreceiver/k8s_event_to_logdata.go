// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Number of log attributes to add to the pdata.LogSlice.
	totalLogAttributes = 9

	// Number of resource attributes to add to the pdata.ResourceLogs.
	totalResourceAttributes = 3
)

// Only two types of events are created as of now.
// For more info: https://docs.openshift.com/container-platform/4.9/rest_api/metadata_apis/event-core-v1.html
var severityMap = map[string]pdata.SeverityNumber{
	"normal":  pdata.SeverityNumberINFO,
	"warning": pdata.SeverityNumberWARN,
}

// k8sEventToLogRecord converts Kubernetes event to pdata.LogSlice and adds the resource attributes.
func k8sEventToLogData(logger *zap.Logger, ev *corev1.Event) pdata.Logs {
	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	lr := ill.Logs().AppendEmpty()

	resourceAttrs := rl.Resource().Attributes()
	resourceAttrs.EnsureCapacity(totalResourceAttributes)

	resourceAttrs.InsertString(semconv.AttributeK8SClusterName, ev.ObjectMeta.ClusterName)
	resourceAttrs.InsertString("k8s.event.source", ev.Source.Component)
	resourceAttrs.InsertString(semconv.AttributeK8SNodeName, ev.Source.Host)

	// The Reason field is the most logical "name" of the event.
	lr.SetName(ev.Reason)
	lr.SetTimestamp(pdata.Timestamp(getEventTimestamp(ev).UnixNano()))

	// The Message field contains description about the event,
	// which is best suited for the "Body" of the LogSlice.
	lr.Body().SetStringVal(ev.Message)

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

	attrs.InsertString("k8s.event.action", ev.Action)
	attrs.InsertString("k8s.event.start_time", ev.ObjectMeta.CreationTimestamp.String())
	attrs.InsertString("k8s.event.name", ev.ObjectMeta.Name)
	attrs.InsertString("k8s.event.uid", string(ev.ObjectMeta.UID))
	attrs.InsertString(semconv.AttributeK8SContainerName, ev.InvolvedObject.FieldPath)
	attrs.InsertString(semconv.AttributeK8SNamespaceName, ev.InvolvedObject.Namespace)

	// "Count" field of k8s event will be '0' in case it is
	// not present in the collected event from k8s.
	if ev.Count != 0 {
		attrs.InsertInt("k8s.event.count", int64(ev.Count))
	}

	involvedObjectName := fmt.Sprintf("k8s.%s.name", strings.ToLower(ev.InvolvedObject.Kind))
	involvedObjectUID := fmt.Sprintf("k8s.%s.uid", strings.ToLower(ev.InvolvedObject.Kind))

	attrs.InsertString(involvedObjectName, ev.InvolvedObject.Name)
	attrs.InsertString(involvedObjectUID, string(ev.InvolvedObject.UID))

	return ld
}
