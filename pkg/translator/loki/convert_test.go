// Copyright The OpenTelemetry Authors
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

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestConvertAttributesAndMerge(t *testing.T) {
	testCases := []struct {
		desc     string
		logAttrs map[string]interface{}
		resAttrs map[string]interface{}
		expected model.LabelSet
	}{
		{
			desc:     "empty attributes should have at least the default labels",
			expected: defaultExporterLabels,
		},
		{
			desc: "selected log attribute should be included",
			logAttrs: map[string]interface{}{
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
			logAttrs: map[string]interface{}{
				hintResources: "host.name",
			},
			resAttrs: map[string]interface{}{
				"host.name": "guarana",
				"pod.name":  "should-be-ignored",
			},
			expected: model.LabelSet{
				"exporter":  "OTLP",
				"host.name": "guarana",
			},
		},
		{
			desc: "selected attributes from both sources should have most specific win",
			logAttrs: map[string]interface{}{
				"host.name":    "hostname-from-attributes",
				hintAttributes: "host.name",
				hintResources:  "host.name",
			},
			resAttrs: map[string]interface{}{
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
			logAttrs: map[string]interface{}{
				hintAttributes: "exporter",
				"exporter":     "overridden",
			},
			expected: model.LabelSet{
				"exporter": "overridden",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			logAttrs := pcommon.NewMap()
			logAttrs.FromRaw(tC.logAttrs)
			resAttrs := pcommon.NewMap()
			resAttrs.FromRaw(tC.resAttrs)
			out := convertAttributesAndMerge(logAttrs, resAttrs)
			assert.Equal(t, tC.expected, out)
		})
	}
}

func TestConvertAttributesToLabels(t *testing.T) {
	attrsToSelectSlice := pcommon.NewValueSlice()
	attrsToSelectSlice.SliceVal().AppendEmpty()
	attrsToSelectSlice.SliceVal().At(0).SetStringVal("host.name")
	attrsToSelectSlice.SliceVal().AppendEmpty()
	attrsToSelectSlice.SliceVal().At(1).SetStringVal("pod.name")

	testCases := []struct {
		desc           string
		attrsAvailable map[string]interface{}
		attrsToSelect  pcommon.Value
		expected       model.LabelSet
	}{
		{
			desc: "string value",
			attrsAvailable: map[string]interface{}{
				"host.name": "guarana",
			},
			attrsToSelect: pcommon.NewValueString("host.name"),
			expected: model.LabelSet{
				"host.name": "guarana",
			},
		},
		{
			desc: "list of values as string",
			attrsAvailable: map[string]interface{}{
				"host.name": "guarana",
				"pod.name":  "pod-123",
			},
			attrsToSelect: pcommon.NewValueString("host.name, pod.name"),
			expected: model.LabelSet{
				"host.name": "guarana",
				"pod.name":  "pod-123",
			},
		},
		{
			desc: "list of values as slice",
			attrsAvailable: map[string]interface{}{
				"host.name": "guarana",
				"pod.name":  "pod-123",
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
			attrsAvailable.FromRaw(tC.attrsAvailable)
			out := convertAttributesToLabels(attrsAvailable, tC.attrsToSelect)
			assert.Equal(t, tC.expected, out)
		})
	}
}

func TestRemoveAttributes(t *testing.T) {
	testCases := []struct {
		desc     string
		attrs    map[string]interface{}
		labels   model.LabelSet
		expected map[string]interface{}
	}{
		{
			desc: "remove hints",
			attrs: map[string]interface{}{
				hintAttributes: "some.field",
				hintResources:  "some.other.field",
				"host.name":    "guarana",
			},
			labels: model.LabelSet{},
			expected: map[string]interface{}{
				"host.name": "guarana",
			},
		},
		{
			desc: "remove attributes promoted to labels",
			attrs: map[string]interface{}{
				"host.name": "guarana",
				"pod.name":  "guarana-123",
			},
			labels: model.LabelSet{
				"host.name": "guarana",
			},
			expected: map[string]interface{}{
				"pod.name": "guarana-123",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			attrs := pcommon.NewMap()
			attrs.FromRaw(tC.attrs)
			removeAttributes(attrs, tC.labels)
			assert.Equal(t, tC.expected, attrs.AsRaw())
		})
	}
}
