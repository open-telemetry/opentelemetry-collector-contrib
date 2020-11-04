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

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func createConfig() *Config {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.Collector = "foocollector"
	config.SourceCategoryPrefix = "prefix/"
	config.SourceCategoryReplaceDash = "#"
	return config
}

var (
	cfg = createConfig()

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
		"_collector":                   "foocollector",
		"_sourceName":                  "namespace-1.pod-5db86d8867-sdqlj.container-1",
		"_sourceCategory":              "prefix/namespace#1/pod",
	}

	k8sNewLabels = map[string]string{
		"k8s.namespace.name":               "namespace-1",
		"k8s.pod.id":                       "pod-1234",
		"k8s.pod.name":                     "pod-5db86d8867-sdqlj",
		"k8s.pod.labels.pod-template-hash": "5db86d8867",
		"k8s.container.name":               "container-1",
	}

	mergedK8sNewLabelsWithMeta = map[string]string{
		"k8s.namespace.name":               "namespace-1",
		"k8s.pod.id":                       "pod-1234",
		"k8s.pod.name":                     "pod-5db86d8867-sdqlj",
		"k8s.pod.labels.pod-template-hash": "5db86d8867",
		"k8s.container.name":               "container-1",
		"k8s.pod.pod_name":                 "pod",
		"_sourceName":                      "namespace-1.pod-5db86d8867-sdqlj.container-1",
		"_sourceCategory":                  "prefix/namespace#1/pod",
		"_collector":                       "foocollector",
	}

	limitedLabels = map[string]string{
		"pod_id": "pod-1234",
	}

	limitedLabelsWithMeta = map[string]string{
		"pod_id":     "pod-1234",
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

func prepareAttributesForAssert(t pdata.Traces) {
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
	prepareAttributesForAssert(t1)
	prepareAttributesForAssert(t2)
	assert.Equal(t, t1, t2)
}

func assertSpansEqual(t *testing.T, t1 pdata.Traces, t2 pdata.Traces) {
	prepareAttributesForAssert(t1)
	prepareAttributesForAssert(t2)
	assert.Equal(t, t1.ResourceSpans().Len(), t2.ResourceSpans().Len())
	for i := 0; i < t1.ResourceSpans().Len(); i++ {
		rss1 := t1.ResourceSpans().At(i)
		rss2 := t2.ResourceSpans().At(i)
		assert.Equal(t, rss1.InstrumentationLibrarySpans(), rss2.InstrumentationLibrarySpans())
	}
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

func TestTraceSourceProcessorNewTaxonomy(t *testing.T) {
	want := newTraceData(mergedK8sNewLabelsWithMeta)
	test := newTraceData(k8sNewLabels)

	ttn := &testTraceConsumer{}
	config := createConfig()
	config.NamespaceKey = "k8s.namespace.name"
	config.PodIDKey = "k8s.pod.id"
	config.PodNameKey = "k8s.pod.pod_name"
	config.PodKey = "k8s.pod.name"
	config.PodTemplateHashKey = "k8s.pod.labels.pod-template-hash"
	config.ContainerKey = "k8s.container.name"

	rtp, _ := newSourceTraceProcessor(ttn, config)
	rtp.ConsumeTraces(context.Background(), test)

	assertTracesEqual(t, ttn.td, want)
}

func TestTraceSourceProcessorEmpty(t *testing.T) {
	want := newTraceData(limitedLabelsWithMeta)
	test := newTraceData(limitedLabels)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)

	rtp.ConsumeTraces(context.Background(), test)
	assertTracesEqual(t, ttn.td, want)
}

func TestTraceSourceProcessorMetaAtSpan(t *testing.T) {
	test := newTraceDataWithSpans(limitedLabels, k8sLabels)
	want := newTraceDataWithSpans(limitedLabelsWithMeta, mergedK8sLabels)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)

	rtp.ConsumeTraces(context.Background(), test)

	assertTracesEqual(t, ttn.td, want)
}

func TestTraceSourceFilteringOutByRegex(t *testing.T) {
	cfg1 := createConfig()
	cfg1.ExcludePodRegex = ".*"

	cfg2 := createConfig()
	cfg2.ExcludeContainerRegex = ".*"

	cfg3 := createConfig()
	cfg3.ExcludeNamespaceRegex = ".*"

	for _, config := range []*Config{cfg1, cfg2, cfg3} {
		test := newTraceDataWithSpans(limitedLabels, k8sLabels)

		want := newTraceDataWithSpans(limitedLabelsWithMeta, mergedK8sLabels)
		want.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(0)

		ttn := &testTraceConsumer{}
		rtp, _ := newSourceTraceProcessor(ttn, config)

		rtp.ConsumeTraces(context.Background(), test)

		assertTracesEqual(t, ttn.td, want)
	}
}

func TestTraceSourceFilteringOutByExclude(t *testing.T) {
	test := newTraceDataWithSpans(limitedLabels, k8sLabels)
	test.ResourceSpans().At(0).Resource().Attributes().UpsertString("pod_annotation_sumologic.com/exclude", "true")

	want := newTraceDataWithSpans(limitedLabelsWithMeta, mergedK8sLabels)
	want.ResourceSpans().At(0).InstrumentationLibrarySpans().Resize(0)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)

	rtp.ConsumeTraces(context.Background(), test)

	assertSpansEqual(t, ttn.td, want)
}

func TestTraceSourceIncludePrecedence(t *testing.T) {
	test := newTraceDataWithSpans(limitedLabels, k8sLabels)
	test.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).Attributes().UpsertString("pod_annotation_sumologic.com/include", "true")

	want := newTraceDataWithSpans(limitedLabelsWithMeta, mergedK8sLabels)
	want.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).Attributes().UpsertString("pod_annotation_sumologic.com/include", "true")

	ttn := &testTraceConsumer{}
	cfg1 := createConfig()
	cfg1.ExcludePodRegex = ".*"
	rtp, _ := newSourceTraceProcessor(ttn, cfg1)

	rtp.ConsumeTraces(context.Background(), test)

	assertSpansEqual(t, ttn.td, want)
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
