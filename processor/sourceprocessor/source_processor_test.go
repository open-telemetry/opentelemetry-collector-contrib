// Copyright 2019 OpenTelemetry Authors
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

package sourceprocessor

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/stretchr/testify/assert"
)

var (
	cfg = &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: "resource",
			NameVal: "resource",
		},
		Collector:                 "foocollector",
		SourceName:                "%{namespace}.%{pod}.%{container}",
		SourceCategory:            "%{namespace}/%{pod_name}",
		SourceCategoryPrefix:      "prefix/",
		SourceCategoryReplaceDash: "#",
	}

	k8sLabels = map[string]string{
		"namespace":                    "namespace-1",
		"pod_id":                       "pod-1234",
		"pod":                          "pod-5db86d8867-sdqlj",
		"pod_labels_pod-template-hash": "5db86d8867",
		"container":                    "container-1",
	}

	mergedK8sLabels = map[string]string{
		"container":                    "container-1",
		"namespace":                    "namespace-1",
		"pod_id":                       "pod-1234",
		"pod":                          "pod-5db86d8867-sdqlj",
		"pod_name":                     "pod",
		"pod_labels_pod-template-hash": "5db86d8867",
		"_sourceName":                  "namespace-1.pod-5db86d8867-sdqlj.container-1",
		"_sourceCategory":              "prefix/namespace#1/pod",
	}

	mergedK8sLabelsWithMeta = map[string]string{
		"container":                    "container-1",
		"namespace":                    "namespace-1",
		"pod_id":                       "pod-1234",
		"pod":                          "pod-5db86d8867-sdqlj",
		"pod_name":                     "pod",
		"pod_labels_pod-template-hash": "5db86d8867",
		"_source":                      "traces",
		"_collector":                   "foocollector",
		"_sourceName":                  "namespace-1.pod-5db86d8867-sdqlj.container-1",
		"_sourceCategory":              "prefix/namespace#1/pod",
	}

	limitedLabels = map[string]string{
		"pod_id": "pod-1234",
	}

	lmitedLabelsWithMeta = map[string]string{
		"pod_id":     "pod-1234",
		"_source":    "traces",
		"_collector": "foocollector",
	}
)

func newTraceData(labels map[string]string) pdata.Traces {
	td := pdata.NewTraces()
	td.ResourceSpans().Resize(1)
	rs := td.ResourceSpans().At(0)
	rs.Resource().InitEmpty()
	attrs := rs.Resource().Attributes()
	for k, v := range labels {
		attrs.UpsertString(k, v)
	}
	return td
}

func prepareAttributes(t pdata.Traces) {
	for i := 0; i < t.ResourceSpans().Len(); i++ {
		rss := t.ResourceSpans().At(i)
		rss.Resource().Attributes().Sort()
		for j := 0; j < rss.InstrumentationLibrarySpans().Len(); j++ {
			ss := rss.InstrumentationLibrarySpans().At(j).Spans()
			for k := 0; k < ss.Len(); k++ {
				ss.At(k).Attributes().Sort()
			}
		}
	}
}

func assertTracesEqual(t *testing.T, t1 pdata.Traces, t2 pdata.Traces) {
	prepareAttributes(t1)
	prepareAttributes(t2)
	assert.Equal(t, t1, t2)
}

func TestTraceSourceProcessor(t *testing.T) {
	want := newTraceData(mergedK8sLabelsWithMeta)
	test := newTraceData(k8sLabels)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)
	assert.True(t, rtp.GetCapabilities().MutatesConsumedData)

	rtp.ConsumeTraces(context.Background(), test)

	assertTracesEqual(t, ttn.td, want)
}

func TestTraceSourceProcessorEmpty(t *testing.T) {
	want := newTraceData(lmitedLabelsWithMeta)
	test := newTraceData(limitedLabels)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)

	rtp.ConsumeTraces(context.Background(), test)
	assertTracesEqual(t, ttn.td, want)
}

func newTraceDataWithSpans(_resourceLabels map[string]string, _spanLabels map[string]string) pdata.Traces {
	// This will be very small attribute set, the actual data will be at span level
	td := newTraceData(_resourceLabels)
	ils := td.ResourceSpans().At(0).InstrumentationLibrarySpans()
	ils.Resize(1)
	spans := ils.At(0).Spans()
	spans.Resize(1)
	spans.At(0).InitEmpty()
	spans.At(0).SetName("foo")
	spanAttrs := spans.At(0).Attributes()
	for k, v := range _spanLabels {
		spanAttrs.UpsertString(k, v)
	}
	return td
}

func TestTraceSourceProcessorMetaAtSpan(t *testing.T) {
	test := newTraceDataWithSpans(limitedLabels, k8sLabels)
	want := newTraceDataWithSpans(lmitedLabelsWithMeta, mergedK8sLabels)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)

	rtp.ConsumeTraces(context.Background(), test)

	assertTracesEqual(t, ttn.td, want)
}

func TestTraceSourceProcessorAnnotations(t *testing.T) {
	k8sLabels["pod_annotation_sumologic.com/sourceHost"] = "sh:%{pod_id}"
	k8sLabels["pod_annotation_sumologic.com/sourceCategory"] = "sc:%{pod_id}"
	test := newTraceData(k8sLabels)

	mergedK8sLabelsWithMeta["pod_annotation_sumologic.com/sourceHost"] = "sh:%{pod_id}"
	mergedK8sLabelsWithMeta["pod_annotation_sumologic.com/sourceCategory"] = "sc:%{pod_id}"
	mergedK8sLabelsWithMeta["_sourceHost"] = "sh:pod-1234"
	mergedK8sLabelsWithMeta["_sourceCategory"] = "prefix/sc:pod#1234"
	want := newTraceData(mergedK8sLabelsWithMeta)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)
	assert.True(t, rtp.GetCapabilities().MutatesConsumedData)

	rtp.ConsumeTraces(context.Background(), test)

	assertTracesEqual(t, ttn.td, want)
}

type testTraceConsumer struct {
	td pdata.Traces
}

func (ttn *testTraceConsumer) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	ttn.td = td
	return nil
}
