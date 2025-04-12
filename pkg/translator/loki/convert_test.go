// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestConvertAttributesAndMerge(t *testing.T) {
	testCases := []struct {
		desc                 string
		logAttrs             map[string]any
		resAttrs             map[string]any
		expected             model.LabelSet
		defaultLabelsEnabled map[string]bool
	}{
		{
			desc:     "empty attributes should have at least the default labels",
			expected: model.LabelSet{"exporter": "OTLP"},
		},
		{
			desc: "selected log attribute should be included",
			logAttrs: map[string]any{
				"host.name":    "guarana",
				"pod.name":     "should-be-ignored",
				hintAttributes: "host.name",
			},
			expected: model.LabelSet{
				"exporter":  "OTLP",
				"host.name": "guarana",
			},
		},
		{
			desc: "selected resource attribute should be included",
			logAttrs: map[string]any{
				hintResources: "host.name",
			},
			resAttrs: map[string]any{
				"host.name": "guarana",
				"pod.name":  "should-be-ignored",
			},
			expected: model.LabelSet{
				"exporter":  "OTLP",
				"host.name": "guarana",
			},
		},
		{
			desc:     "selected attributes from resource attributes should be included",
			logAttrs: map[string]any{},
			resAttrs: map[string]any{
				hintResources: "host.name",
				"host.name":   "hostname-from-resources",
				"pod.name":    "should-be-ignored",
			},
			expected: model.LabelSet{
				"exporter":  "OTLP",
				"host.name": "hostname-from-resources",
			},
		},
		{
			desc: "selected attributes from both sources should have most specific win",
			logAttrs: map[string]any{
				"host.name":    "hostname-from-attributes",
				hintAttributes: "host.name",
				hintResources:  "host.name",
			},
			resAttrs: map[string]any{
				"host.name": "hostname-from-resources",
				"pod.name":  "should-be-ignored",
			},
			expected: model.LabelSet{
				"exporter":  "OTLP",
				"host.name": "hostname-from-attributes",
			},
		},
		{
			desc: "it should be possible to override the exporter label",
			logAttrs: map[string]any{
				hintAttributes: "exporter",
				"exporter":     "overridden",
			},
			expected: model.LabelSet{
				"exporter": "overridden",
			},
		},
		{
			desc: "it should add service.namespace/service.name as job label if both of them are present",
			resAttrs: map[string]any{
				"service.namespace": "my-service-namespace",
				"service.name":      "my-service-name",
			},
			expected: model.LabelSet{
				"exporter": "OTLP",
				"job":      "my-service-namespace/my-service-name",
			},
		},
		{
			desc: "it should add service.name as job label if service.namespace is missing",
			resAttrs: map[string]any{
				"service.name": "my-service-name",
			},
			expected: model.LabelSet{
				"exporter": "OTLP",
				"job":      "my-service-name",
			},
		},
		{
			desc: "it shouldn't add service.namespace as job label if service.name is missing",
			resAttrs: map[string]any{
				"service.namespace": "my-service-namespace",
			},
			expected: model.LabelSet{
				"exporter": "OTLP",
			},
		},
		{
			desc: "it should add service.instance.id as instance label if service.instance.id is present",
			resAttrs: map[string]any{
				"service.instance.id": "my-service-instance-id",
			},
			expected: model.LabelSet{
				"exporter": "OTLP",
				"instance": "my-service-instance-id",
			},
		},
		{
			desc: "it shouldn't add job, instance, exporter labels if they disabled in config",
			resAttrs: map[string]any{
				"service.instance.id": "my-service-instance-id",
				"service.namespace":   "my-service-namespace",
				"service.name":        "my-service-name",
			},
			defaultLabelsEnabled: map[string]bool{
				exporterLabel:       false,
				model.JobLabel:      false,
				model.InstanceLabel: false,
			},
			expected: model.LabelSet{},
		},
		{
			desc: "it should add job label because it is enabled in config, and exporter label because it is not mentioned in config and that's why enabled by default",
			resAttrs: map[string]any{
				"service.instance.id": "my-service-instance-id",
				"service.namespace":   "my-service-namespace",
				"service.name":        "my-service-name",
			},
			defaultLabelsEnabled: map[string]bool{
				model.JobLabel:      true,
				model.InstanceLabel: false,
			},
			expected: model.LabelSet{
				"job":      "my-service-namespace/my-service-name",
				"exporter": "OTLP",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			logAttrs := pcommon.NewMap()
			assert.NoError(t, logAttrs.FromRaw(tC.logAttrs))
			resAttrs := pcommon.NewMap()
			assert.NoError(t, resAttrs.FromRaw(tC.resAttrs))
			out := convertAttributesAndMerge(logAttrs, resAttrs, tC.defaultLabelsEnabled)
			assert.Equal(t, tC.expected, out)
		})
	}
}

func TestConvertAttributesToLabels(t *testing.T) {
	attrsToSelectSlice := pcommon.NewValueSlice()
	attrsToSelectSlice.Slice().AppendEmpty()
	attrsToSelectSlice.Slice().At(0).SetStr("host.name")
	attrsToSelectSlice.Slice().AppendEmpty()
	attrsToSelectSlice.Slice().At(1).SetStr("pod.name")

	testCases := []struct {
		desc           string
		attrsAvailable map[string]any
		attrsToSelect  pcommon.Value
		expected       model.LabelSet
	}{
		{
			desc: "string value",
			attrsAvailable: map[string]any{
				"host.name": "guarana",
			},
			attrsToSelect: pcommon.NewValueStr("host.name"),
			expected: model.LabelSet{
				"host.name": "guarana",
			},
		},
		{
			desc: "list of values as string",
			attrsAvailable: map[string]any{
				"host.name": "guarana",
				"pod.name":  "pod-123",
			},
			attrsToSelect: pcommon.NewValueStr("host.name, pod.name"),
			expected: model.LabelSet{
				"host.name": "guarana",
				"pod.name":  "pod-123",
			},
		},
		{
			desc: "list of values as slice",
			attrsAvailable: map[string]any{
				"host.name": "guarana",
				"pod.name":  "pod-123",
			},
			attrsToSelect: attrsToSelectSlice,
			expected: model.LabelSet{
				"host.name": "guarana",
				"pod.name":  "pod-123",
			},
		},
		{
			desc: "nested attributes",
			attrsAvailable: map[string]any{
				"host": map[string]any{
					"name": "guarana",
				},
				"pod.name": "pod-123",
			},
			attrsToSelect: attrsToSelectSlice,
			expected: model.LabelSet{
				"host.name": "guarana",
				"pod.name":  "pod-123",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			attrsAvailable := pcommon.NewMap()
			assert.NoError(t, attrsAvailable.FromRaw(tC.attrsAvailable))
			out := convertAttributesToLabels(attrsAvailable, tC.attrsToSelect)
			assert.Equal(t, tC.expected, out)
		})
	}
}

func TestRemoveAttributes(t *testing.T) {
	testCases := []struct {
		desc     string
		attrs    map[string]any
		labels   model.LabelSet
		expected map[string]any
	}{
		{
			desc: "remove hints",
			attrs: map[string]any{
				hintAttributes: "some.field",
				hintResources:  "some.other.field",
				hintFormat:     "logfmt",
				hintTenant:     "some_tenant",
				"host.name":    "guarana",
			},
			labels: model.LabelSet{},
			expected: map[string]any{
				"host.name": "guarana",
			},
		},
		{
			desc: "remove attributes promoted to labels",
			attrs: map[string]any{
				"host.name": "guarana",
				"pod.name":  "guarana-123",
			},
			labels: model.LabelSet{
				"host.name": "guarana",
			},
			expected: map[string]any{
				"pod.name": "guarana-123",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			attrs := pcommon.NewMap()
			assert.NoError(t, attrs.FromRaw(tC.attrs))
			removeAttributes(attrs, tC.labels)
			assert.Equal(t, tC.expected, attrs.AsRaw())
		})
	}
}

func TestGetAttribute(t *testing.T) {
	testCases := []struct {
		desc     string
		attr     string
		attrs    map[string]any
		expected pcommon.Value
		ok       bool
	}{
		{
			desc: "attributes don't contain dotted names",
			attr: "host.name",
			attrs: map[string]any{
				"host": map[string]any{
					"name": "guarana",
				},
			},
			expected: pcommon.NewValueStr("guarana"),
			ok:       true,
		},
		{
			desc: "attributes contain dotted name on the nested level",
			attr: "log.file.name",
			attrs: map[string]any{
				"log": map[string]any{
					"file.name": "foo",
				},
			},
			expected: pcommon.NewValueStr("foo"),
			ok:       true,
		},
		{
			desc: "attributes contain dotted name on the upper level",
			attr: "log.file.name",
			attrs: map[string]any{
				"log.file": map[string]any{
					"name": "foo",
				},
			},
			expected: pcommon.NewValueStr("foo"),
			ok:       true,
		},
		{
			desc: "attributes contain dotted attribute",
			attr: "log.file.name",
			attrs: map[string]any{
				"log.file.name": "foo",
			},
			expected: pcommon.NewValueStr("foo"),
			ok:       true,
		},
		{
			desc: "dotted name that doesn't match attr",
			attr: "log.file.name",
			attrs: map[string]any{
				"log.file": "foo",
			},
			expected: pcommon.Value{},
			ok:       false,
		},
		{
			desc: "should get the longest match",
			attr: "log.file.name",
			attrs: map[string]any{
				"log.file.name": "foo",
				"log": map[string]any{
					"file": map[string]any{
						"name": "bar",
					},
				},
				"log.file": map[string]any{
					"name": "baz",
				},
			},
			expected: pcommon.NewValueStr("foo"),
			ok:       true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			// prepare
			attrs := pcommon.NewMap()
			err := attrs.FromRaw(tC.attrs)
			require.NoError(t, err)

			// test
			attr, ok := getAttribute(tC.attr, attrs)

			// verify
			assert.Equal(t, tC.expected, attr)
			assert.Equal(t, tC.ok, ok)
		})
	}
}

func TestConvertLogToLogRawEntry(t *testing.T) {
	log, _, _ := exampleLog()
	log.SetTimestamp(pcommon.NewTimestampFromTime(timeNow()))

	expectedLogEntry := &push.Entry{
		Timestamp: timestampFromLogRecord(log),
		Line:      "Example log",
	}

	out, err := convertLogToLogRawEntry(log)
	assert.NoError(t, err)
	assert.Equal(t, expectedLogEntry, out)
}
