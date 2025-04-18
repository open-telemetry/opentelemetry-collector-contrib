// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/kube"
)

func TestK8sEventToLogData(t *testing.T) {
	k8sEvent := getEvent()

	ld := k8sEventToLogData(zap.NewNop(), k8sEvent)
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	lr := rl.ScopeLogs().At(0)
	attrs := lr.LogRecords().At(0).Attributes()
	assert.Equal(t, 1, ld.ResourceLogs().Len())
	assert.Equal(t, 7, resourceAttrs.Len())
	assert.Equal(t, 7, attrs.Len())

	// Count attribute will not be present in the LogData
	k8sEvent.Count = 0
	ld = k8sEventToLogData(zap.NewNop(), k8sEvent)
	assert.Equal(t, 6, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len())
}

func TestK8sEventToLogDataWithApiAndResourceVersion(t *testing.T) {
	k8sEvent := getEvent()

	ld := k8sEventToLogData(zap.NewNop(), k8sEvent)
	attrs := ld.ResourceLogs().At(0).Resource().Attributes()
	attr, ok := attrs.Get("k8s.object.api_version")
	assert.True(t, ok)
	assert.Equal(t, "v1", attr.AsString())

	attr, ok = attrs.Get("k8s.object.resource_version")
	assert.True(t, ok)
	assert.Empty(t, attr.AsString())

	// add ResourceVersion
	k8sEvent.InvolvedObject.ResourceVersion = "7387066320"
	ld = k8sEventToLogData(zap.NewNop(), k8sEvent)
	attrs = ld.ResourceLogs().At(0).Resource().Attributes()
	attr, ok = attrs.Get("k8s.object.resource_version")
	assert.True(t, ok)
	assert.Equal(t, "7387066320", attr.AsString())
}

func TestK8sEventToLogDataWithExtractionRules(t *testing.T) {
	k8sEvent := getEvent()
	k8sEvent.Labels["testlabel"] = "test-key-label"
	k8sEvent.Labels["app.kubernetes.io/by-key"] = "test-key-label"
	k8sEvent.Labels["app.kubernetes.io/by-regex"] = "test-regex-label"
	k8sEvent.Annotations["topology.kubernetes.io/by-key"] = "test-key-annotation"
	k8sEvent.Annotations["topology.kubernetes.io/by-regex"] = "test-regex-annotation"

	labelsRules, err := ExtractFieldRules("labels",
		FieldExtractConfig{
			Key: "testlabel",
		},
		FieldExtractConfig{
			TagName: "k8s.event.labels.label_by_key",
			Key:     "app.kubernetes.io/by-key",
		},
		FieldExtractConfig{
			TagName:  "k8s.event.labels.$1",
			KeyRegex: "app.kubernetes.io/(.*)",
		})
	assert.NoError(t, err)

	annotationsRules, err := ExtractFieldRules("annotations", FieldExtractConfig{
		TagName: "k8s.event.annotations.annotation_by_key",
		Key:     "topology.kubernetes.io/by-key",
	}, FieldExtractConfig{
		TagName:  "k8s.event.annotations.$1",
		KeyRegex: "topology.kubernetes.io/(.*)",
	})
	assert.NoError(t, err)

	ld := k8sEventToLogData(zap.NewNop(), k8sEvent, &kube.ExtractionRules{
		Labels:      labelsRules,
		Annotations: annotationsRules,
	})
	assert.NoError(t, err)

	rl := ld.ResourceLogs().At(0)
	lr := rl.ScopeLogs().At(0)
	attrs := lr.LogRecords().At(0).Attributes()

	attr, ok := attrs.Get("k8s.event.labels.testlabel")
	assert.True(t, ok)
	assert.Equal(t, "test-key-label", attr.AsString())

	attr, ok = attrs.Get("k8s.event.labels.label_by_key")
	assert.True(t, ok)
	assert.Equal(t, "test-key-label", attr.AsString())

	attr, ok = attrs.Get("k8s.event.labels.by-regex")
	assert.True(t, ok)
	assert.Equal(t, "test-regex-label", attr.AsString())

	attr, ok = attrs.Get("k8s.event.annotations.annotation_by_key")
	assert.True(t, ok)
	assert.Equal(t, "test-key-annotation", attr.AsString())

	attr, ok = attrs.Get("k8s.event.annotations.by-regex")
	assert.True(t, ok)
	assert.Equal(t, "test-regex-annotation", attr.AsString())
}

func TestUnknownSeverity(t *testing.T) {
	k8sEvent := getEvent()
	k8sEvent.Type = "Unknown"

	ld := k8sEventToLogData(zap.NewNop(), k8sEvent)
	rl := ld.ResourceLogs().At(0)
	logEntry := rl.ScopeLogs().At(0).LogRecords().At(0)

	assert.Equal(t, plog.SeverityNumberUnspecified, logEntry.SeverityNumber())
	assert.Empty(t, logEntry.SeverityText())
}
