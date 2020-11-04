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
	"go.opentelemetry.io/collector/model/pdata"
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
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	for k, v := range labels {
		attrs.UpsertString(k, v)
	}
	return td
}

func newTraceDataWithSpans(_resourceLabels map[string]string, _spanLabels map[string]string) pdata.Traces {
	// This will be very small attribute set, the actual data will be at span level
	td := newTraceData(_resourceLabels)
	ils := td.ResourceSpans().At(0).InstrumentationLibrarySpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetName("foo")
	spanAttrs := span.Attributes()
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

	rtp := newSourceTraceProcessor(cfg)

	td, err := rtp.ProcessTraces(context.Background(), test)
	assert.NoError(t, err)

	assertTracesEqual(t, td, want)
}

func TestTraceSourceProcessorNewTaxonomy(t *testing.T) {
	want := newTraceData(mergedK8sNewLabelsWithMeta)
	test := newTraceData(k8sNewLabels)

	config := createConfig()
	config.NamespaceKey = "k8s.namespace.name"
	config.PodIDKey = "k8s.pod.id"
	config.PodNameKey = "k8s.pod.pod_name"
	config.PodKey = "k8s.pod.name"
	config.PodTemplateHashKey = "k8s.pod.labels.pod-template-hash"
	config.ContainerKey = "k8s.container.name"

	rtp := newSourceTraceProcessor(config)

	td, err := rtp.ProcessTraces(context.Background(), test)
	assert.NoError(t, err)

	assertTracesEqual(t, td, want)
}

func TestTraceSourceProcessorEmpty(t *testing.T) {
	want := newTraceData(limitedLabelsWithMeta)
	test := newTraceData(limitedLabels)

	rtp := newSourceTraceProcessor(cfg)

	td, err := rtp.ProcessTraces(context.Background(), test)
	assert.NoError(t, err)
	assertTracesEqual(t, td, want)
}

func TestTraceSourceFilteringOutByRegex(t *testing.T) {
	testcases := []struct {
		name string
		cfg  *Config
		want pdata.Traces
	}{
		{
			name: "pod exclude regex",
			cfg: func() *Config {
				conf := createConfig()
				conf.ExcludePodRegex = ".*"
				return conf
			}(),
			want: func() pdata.Traces {
				want := newTraceDataWithSpans(mergedK8sLabelsWithMeta, k8sLabels)
				want.ResourceSpans().At(0).InstrumentationLibrarySpans().
					RemoveIf(func(pdata.InstrumentationLibrarySpans) bool { return true })
				return want
			}(),
		},
		{
			name: "container exclude regex",
			cfg: func() *Config {
				conf := createConfig()
				conf.ExcludeContainerRegex = ".*"
				return conf
			}(),
			want: func() pdata.Traces {
				want := newTraceDataWithSpans(mergedK8sLabelsWithMeta, k8sLabels)
				want.ResourceSpans().At(0).InstrumentationLibrarySpans().
					RemoveIf(func(pdata.InstrumentationLibrarySpans) bool { return true })
				return want
			}(),
		},
		{
			name: "namespace exclude regex",
			cfg: func() *Config {
				conf := createConfig()
				conf.ExcludeNamespaceRegex = ".*"
				return conf
			}(),
			want: func() pdata.Traces {
				want := newTraceDataWithSpans(mergedK8sLabelsWithMeta, k8sLabels)
				want.ResourceSpans().At(0).InstrumentationLibrarySpans().
					RemoveIf(func(pdata.InstrumentationLibrarySpans) bool { return true })
				return want
			}(),
		},
		{
			name: "no exclude regex",
			cfg: func() *Config {
				return createConfig()
			}(),
			want: func() pdata.Traces {
				return newTraceDataWithSpans(mergedK8sLabelsWithMeta, k8sLabels)
			}(),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			test := newTraceDataWithSpans(mergedK8sLabels, k8sLabels)

			rtp := newSourceTraceProcessor(tc.cfg)

			td, err := rtp.ProcessTraces(context.Background(), test)
			assert.NoError(t, err)

			assertTracesEqual(t, td, tc.want)
		})
	}
}

func TestTraceSourceFilteringOutByExclude(t *testing.T) {
	test := newTraceDataWithSpans(k8sLabels, k8sLabels)
	test.ResourceSpans().At(0).Resource().Attributes().
		UpsertString("pod_annotation_sumologic.com/exclude", "true")

	want := newTraceDataWithSpans(limitedLabelsWithMeta, mergedK8sLabels)
	want.ResourceSpans().At(0).InstrumentationLibrarySpans().
		RemoveIf(func(pdata.InstrumentationLibrarySpans) bool { return true })

	rtp := newSourceTraceProcessor(cfg)

	td, err := rtp.ProcessTraces(context.Background(), test)
	assert.NoError(t, err)

	assertSpansEqual(t, td, want)
}

func TestTraceSourceIncludePrecedence(t *testing.T) {
	test := newTraceDataWithSpans(limitedLabels, k8sLabels)
	test.ResourceSpans().At(0).Resource().Attributes().UpsertString("pod_annotation_sumologic.com/include", "true")

	want := newTraceDataWithSpans(limitedLabelsWithMeta, k8sLabels)
	want.ResourceSpans().At(0).Resource().Attributes().UpsertString("pod_annotation_sumologic.com/include", "true")

	cfg1 := createConfig()
	cfg1.ExcludePodRegex = ".*"
	rtp := newSourceTraceProcessor(cfg)

	td, err := rtp.ProcessTraces(context.Background(), test)
	assert.NoError(t, err)

	assertTracesEqual(t, td, want)
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

	rtp := newSourceTraceProcessor(cfg)

	td, err := rtp.ProcessTraces(context.Background(), test)
	assert.NoError(t, err)

	assertTracesEqual(t, td, want)
}
