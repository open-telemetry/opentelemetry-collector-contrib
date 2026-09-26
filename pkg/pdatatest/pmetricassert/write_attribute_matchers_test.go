// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestWriteAssertionFile_AttributeMatchers(t *testing.T) {
	metrics := buildAttributeMatcherMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, metrics,
		WithAttributeExists("resource.id", "datapoint.id"),
		WithAttributeRegex(map[string]string{
			"host.name":  `worker-[0-9]+`,
			"request.id": `request-[0-9]+`,
		}),
	))

	doc, err := readDocument(path)
	require.NoError(t, err)
	require.Len(t, doc.Resources, 2)
	for _, resource := range doc.Resources {
		require.Equal(t, map[string]any{
			"host.name/regex":    `worker-[0-9]+`,
			"resource.id/exists": true,
			"service.name":       "svc",
		}, resource.Attributes)
		datapoints := resource.Scopes[0].Metrics[0].Datapoints
		require.Len(t, datapoints, 2)
		for _, datapoint := range datapoints {
			require.Equal(t, map[string]any{
				"datapoint.id/exists": true,
				"request.id/regex":    `request-[0-9]+`,
			}, datapoint.Attributes)
		}
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		resourceSuffix := strconv.Itoa(i + 10)
		rm.Resource().Attributes().PutStr("resource.id", "changed-resource-"+resourceSuffix)
		rm.Resource().Attributes().PutStr("host.name", "worker-"+resourceSuffix)
		dps := rm.ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
		for j := 0; j < dps.Len(); j++ {
			datapointSuffix := resourceSuffix + strconv.Itoa(j+10)
			dps.At(j).Attributes().PutStr("datapoint.id", "changed-datapoint-"+datapointSuffix)
			dps.At(j).Attributes().PutStr("request.id", "request-"+datapointSuffix)
		}
	}
	require.NoError(t, AssertMetrics(path, metrics))
}

func TestWriteAssertionFile_AttributeMatchersDefaultUnchanged(t *testing.T) {
	metrics := buildAttributeMatcherMetrics()
	path := filepath.Join(t.TempDir(), "metrics.assert.yaml")
	require.NoError(t, WriteAssertionFile(t, path, metrics))

	doc, err := readDocument(path)
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"host.name":    "worker-1",
		"resource.id":  "resource-1",
		"service.name": "svc",
	}, doc.Resources[0].Attributes)
	require.Equal(t, map[string]any{
		"datapoint.id": "datapoint-11",
		"request.id":   "request-11",
	}, doc.Resources[0].Scopes[0].Metrics[0].Datapoints[0].Attributes)
}

func TestWriteAssertionFile_AttributeRegexValidation(t *testing.T) {
	t.Run("full string match", func(t *testing.T) {
		metrics := buildAttributeMatcherMetrics()
		datapoints := metrics.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
		datapoints.At(1).Attributes().PutStr("request.id", "request-22-extra")
		err := WriteAssertionFile(t, filepath.Join(t.TempDir(), "metrics.assert.yaml"), metrics,
			WithAttributeRegex(map[string]string{"request.id": `request-[0-9]+`}),
		)
		require.ErrorContains(t, err, `attribute "request.id" value "request-22-extra" does not match regex "request-[0-9]+"`)
	})

	t.Run("string attribute", func(t *testing.T) {
		metrics := buildAttributeMatcherMetrics()
		metrics.ResourceMetrics().At(0).Resource().Attributes().PutInt("host.name", 1)
		err := WriteAssertionFile(t, filepath.Join(t.TempDir(), "metrics.assert.yaml"), metrics,
			WithAttributeRegex(map[string]string{"host.name": `worker-[0-9]+`}),
		)
		require.ErrorContains(t, err, `attribute "host.name" must be a string to match /regex`)
	})

	t.Run("pattern syntax", func(t *testing.T) {
		err := WriteAssertionFile(t, filepath.Join(t.TempDir(), "metrics.assert.yaml"), buildAttributeMatcherMetrics(),
			WithAttributeRegex(map[string]string{"missing": `[`}),
		)
		require.ErrorContains(t, err, `attribute "missing"/regex has invalid pattern "["`)
	})
}

func TestWriteAssertionFile_AttributeMatcherConflict(t *testing.T) {
	err := WriteAssertionFile(t, filepath.Join(t.TempDir(), "metrics.assert.yaml"), buildAttributeMatcherMetrics(),
		WithAttributeExists("host.name"),
		WithAttributeRegex(map[string]string{"host.name": `worker-[0-9]+`}),
	)
	require.EqualError(t, err, `attribute "host.name" cannot use both /exists and /regex`)
}

func TestWriteAssertionFile_AttributeMatcherKeyCollision(t *testing.T) {
	metrics := buildAttributeMatcherMetrics()
	metrics.ResourceMetrics().At(0).Resource().Attributes().PutStr("host.name/regex", "exact-value")
	err := WriteAssertionFile(t, filepath.Join(t.TempDir(), "metrics.assert.yaml"), metrics,
		WithAttributeRegex(map[string]string{"host.name": `worker-[0-9]+`}),
	)
	require.ErrorContains(t, err, `cannot generate matcher for attribute "host.name": attribute "host.name/regex" already exists`)
}

func buildAttributeMatcherMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	for i, suffix := range []string{"1", "2"} {
		rm := metrics.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", "svc")
		rm.Resource().Attributes().PutStr("resource.id", "resource-"+suffix)
		rm.Resource().Attributes().PutStr("host.name", "worker-"+suffix)
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("scope")
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("requests")
		dps := metric.SetEmptyGauge().DataPoints()
		for j := 1; j <= 2; j++ {
			dp := dps.AppendEmpty()
			dp.Attributes().PutStr("datapoint.id", "datapoint-"+suffix+strconv.Itoa(j))
			dp.Attributes().PutStr("request.id", "request-"+suffix+strconv.Itoa(j))
			dp.SetIntValue(int64(i + j))
		}
	}
	return metrics
}
