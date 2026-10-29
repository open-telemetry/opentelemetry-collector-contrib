// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/metadata"
)

func TestK8sEventToLogDataWithDifferentEventTypes(t *testing.T) {
	tests := []struct {
		name        string
		eventType   string
		expectedLog plog.SeverityNumber
	}{
		{
			name:        "Normal",
			eventType:   "Normal",
			expectedLog: plog.SeverityNumberInfo,
		},
		{
			name:        "Warning",
			eventType:   "Warning",
			expectedLog: plog.SeverityNumberWarn,
		},
		{
			name:        "Error",
			eventType:   "Error",
			expectedLog: plog.SeverityNumberError,
		},
		{
			name:        "Critical",
			eventType:   "Critical",
			expectedLog: plog.SeverityNumberFatal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			k8sEvent := getEvent(test.eventType)

			ld := k8sEventToLogData(zap.NewNop(), k8sEvent, "latest")
			rl := ld.ResourceLogs().At(0)
			lr := rl.ScopeLogs().At(0)
			logRecord := lr.LogRecords().At(0)

			assert.Equal(t, test.expectedLog, logRecord.SeverityNumber())
		})
	}
}

func TestK8sEventToLogData(t *testing.T) {
	k8sEvent := getEvent("Normal")

	ld := k8sEventToLogData(zap.NewNop(), k8sEvent, "latest")
	rl := ld.ResourceLogs().At(0)
	resourceAttrs := rl.Resource().Attributes()
	lr := rl.ScopeLogs().At(0)
	attrs := lr.LogRecords().At(0).Attributes()
	assert.Equal(t, 1, ld.ResourceLogs().Len())
	assert.Equal(t, 7, resourceAttrs.Len())
	assert.Equal(t, 8, attrs.Len())

	// Count attribute will not be present in the LogData when Series is nil.
	k8sEvent.Series = nil
	ld = k8sEventToLogData(zap.NewNop(), k8sEvent, "latest")
	assert.Equal(t, 7, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len())
}

func TestK8sEventToLogDataWithApiAndResourceVersion(t *testing.T) {
	k8sEvent := getEvent("Normal")

	ld := k8sEventToLogData(zap.NewNop(), k8sEvent, "latest")
	attrs := ld.ResourceLogs().At(0).Resource().Attributes()
	attr, ok := attrs.Get("k8s.object.api_version")
	assert.True(t, ok)
	assert.Equal(t, "v1", attr.AsString())

	attr, ok = attrs.Get("k8s.object.resource_version")
	assert.True(t, ok)
	assert.Empty(t, attr.AsString())

	// add ResourceVersion
	k8sEvent.Regarding.ResourceVersion = "7387066320"
	ld = k8sEventToLogData(zap.NewNop(), k8sEvent, "latest")
	attrs = ld.ResourceLogs().At(0).Resource().Attributes()
	attr, ok = attrs.Get("k8s.object.resource_version")
	assert.True(t, ok)
	assert.Equal(t, "7387066320", attr.AsString())
}

func TestExposingReportingControllerAndInstance(t *testing.T) {
	k8sEvent := getEvent("Normal")
	ld := k8sEventToLogData(zap.NewNop(), k8sEvent, "latest")
	rl := ld.ResourceLogs().At(0)
	logEntry := rl.ScopeLogs().At(0).LogRecords().At(0)

	value, ok := logEntry.Attributes().Get("k8s.event.reporting_controller")
	assert.True(t, ok)
	assert.Equal(t, "some-controller", value.Str())

	value, ok = logEntry.Attributes().Get("k8s.event.reporting_instance")
	assert.True(t, ok)
	assert.Equal(t, "some-instance", value.Str())
}

func TestUnknownSeverity(t *testing.T) {
	k8sEvent := getEvent("Unknown")

	ld := k8sEventToLogData(zap.NewNop(), k8sEvent, "latest")
	rl := ld.ResourceLogs().At(0)
	logEntry := rl.ScopeLogs().At(0).LogRecords().At(0)

	assert.Equal(t, plog.SeverityNumberUnspecified, logEntry.SeverityNumber())
	assert.Empty(t, logEntry.SeverityText())
}

func TestScopeNameAndVersion(t *testing.T) {
	k8sEvent := getEvent("Normal")

	version := "latest"
	ld := k8sEventToLogData(zap.NewNop(), k8sEvent, version)
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)

	assert.Equal(t, metadata.ScopeName, sl.Scope().Name())
	assert.Equal(t, version, sl.Scope().Version())
}

func TestK8sEventToLogDataLegacyRecorderFallback(t *testing.T) {
	first := time.Now().Add(-time.Hour).Truncate(time.Second)
	last := time.Now().Truncate(time.Second)

	k8sEvent := getEvent("Normal")
	k8sEvent.Series = nil
	k8sEvent.EventTime = metav1.MicroTime{}
	k8sEvent.DeprecatedFirstTimestamp = metav1.Time{Time: first}
	k8sEvent.DeprecatedLastTimestamp = metav1.Time{Time: last}
	k8sEvent.DeprecatedCount = 5

	lr := k8sEventToLogData(zap.NewNop(), k8sEvent, "latest").ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, pcommon.NewTimestampFromTime(last), lr.Timestamp())
	count, ok := lr.Attributes().Get("k8s.event.count")
	require.True(t, ok)
	assert.Equal(t, int64(5), count.Int())

	k8sEvent.DeprecatedLastTimestamp = metav1.Time{}
	lr = k8sEventToLogData(zap.NewNop(), k8sEvent, "latest").ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, pcommon.NewTimestampFromTime(first), lr.Timestamp())
}
