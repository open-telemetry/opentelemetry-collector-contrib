// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func TestReviewRegressionPartitionKeepsRenderedName(t *testing.T) {
	cfg := newPartitionTestConfig("{{ .LogRecordCount }}.json", true)
	cfg.AppendBlob.Enabled = false
	exporter := newAzureBlobExporter(cfg, zaptest.NewLogger(t), pipeline.SignalLogs)
	require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
	client := newRecordingAzBlobClient(nil)
	exporter.client = client

	logs := generateLogsWithActivities("a", "b", "c")
	logs.ResourceLogs().At(2).ScopeLogs().At(0).LogRecords().AppendEmpty().
		Body().SetStr("second log for c")
	require.NoError(t, exporter.ConsumeLogs(t.Context(), logs))

	// The singleton counts are 1, 1, 2. Combining the first two resources must
	// not change their destination from 1.json to 2.json.
	assert.Len(t, client.uploads, 2, "expected two distinct blob destinations")
	assert.Len(t, client.uploads["1.json"], 1)
	assert.Len(t, client.uploads["2.json"], 1)
	assert.Equal(t, []string{"log for a", "log for b"},
		uploadedLogBodies(t, client.uploads["1.json"]))
	assert.Equal(t, []string{"log for c", "second log for c"},
		uploadedLogBodies(t, client.uploads["2.json"]))
}

func TestReviewRegressionLaterResourceTemplateErrorFallsBack(t *testing.T) {
	const nameTemplate = `{{ index (getResourceLogAttr . 0 "parts") 1 }}.json`
	cfg := newPartitionTestConfig(nameTemplate, true)
	core, observed := observer.New(zap.WarnLevel)
	exporter := newAzureBlobExporter(cfg, zap.New(core), pipeline.SignalLogs)
	require.NoError(t, exporter.start(t.Context(), componenttest.NewNopHost()))
	client := newRecordingAzBlobClient(nil)
	exporter.client = client

	logs := generateLogsWithActivities("a", "b")
	first := logs.ResourceLogs().At(0).Resource().Attributes().PutEmptySlice("parts")
	first.AppendEmpty().SetStr("prefix")
	first.AppendEmpty().SetStr("tenant-a")
	second := logs.ResourceLogs().At(1).Resource().Attributes().PutEmptySlice("parts")
	second.AppendEmpty().SetStr("prefix")
	require.NoError(t, exporter.ConsumeLogs(t.Context(), logs))

	// Only the second resource fails. Retrying the template on the original
	// payload would hide that error by reading the first resource again.
	assert.Len(t, client.uploads, 1)
	assert.Empty(t, client.uploads["tenant-a.json"])
	assert.Len(t, client.uploads[nameTemplate], 1)
	assert.Equal(t, []string{"log for a", "log for b"},
		uploadedLogBodies(t, client.uploads[nameTemplate]))
	assert.Equal(t, 1, observed.FilterMessage(
		"Failed to execute blob name template, using default blob name format").Len())
}
