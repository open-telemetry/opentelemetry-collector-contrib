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
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
)

type sourceTraceProcessor struct {
	collector             string
	sourceCategoryFiller  attributeFiller
	sourceNameFiller      attributeFiller
	sourceHostFiller      attributeFiller
	excludeNamespaceRegex *regexp.Regexp
	excludePodRegex       *regexp.Regexp
	excludeContainerRegex *regexp.Regexp
	excludeHostRegex      *regexp.Regexp
	nextConsumer          consumer.TraceConsumer
}

type attributeFiller struct {
	name            string
	compiledFormat  string
	dashReplacement string
	prefix          string
	labels          []string
}

const (
	alphanums = "bcdfghjklmnpqrstvwxz2456789"

	// The namespace used here makes assumptions on what is the configuration of K8S metadata tagging!
	annotationPrefix   = "pod_annotation_"
	podTemplateHashKey = "pod_labels_pod-template-hash"
	podNameKey         = "pod_name"
	namespaceKey       = "namespace"
	podKey             = "pod"
	containerKey       = "container"
	sourceHostKey      = "source_host"

	sourceHostSpecialAnnotation     = annotationPrefix + "sumologic.com/sourceHost"
	sourceNameSpecialAnnotation     = annotationPrefix + "sumologic.com/sourceName"
	sourceCategorySpecialAnnotation = annotationPrefix + "sumologic.com/sourceCategory"

	includeAnnotation = annotationPrefix + "sumologic.com/include"
	excludeAnnotation = annotationPrefix + "sumologic.com/exclude"
)

func compileRegex(regex string) *regexp.Regexp {
	if regex == "" {
		return nil
	}

	re, err := regexp.Compile(regex)
	if err != nil {
		log.Fatalf("Cannot compile reguler expression: %s Error: %v\n", regex, err)
	}

	return re
}

func matchRegexMaybe(re *regexp.Regexp, atts pdata.AttributeMap, attributeName string) bool {
	if re == nil {
		return false
	}

	if attrValue, found := atts.Get(attributeName); found {
		if attrValue.Type() == pdata.AttributeValueSTRING {
			return re.MatchString(attrValue.StringVal())
		}
	}

	return false
}

func newSourceTraceProcessor(next consumer.TraceConsumer, cfg *Config) (*sourceTraceProcessor, error) {
	return &sourceTraceProcessor{
		nextConsumer:          next,
		collector:             cfg.Collector,
		sourceHostFiller:      createSourceHostFiller(),
		sourceCategoryFiller:  createSourceCategoryFiller(cfg),
		sourceNameFiller:      createSourceNameFiller(cfg),
		excludeNamespaceRegex: compileRegex(cfg.ExcludeNamespaceRegex),
		excludeHostRegex:      compileRegex(cfg.ExcludeHostRegex),
		excludeContainerRegex: compileRegex(cfg.ExcludeContainerRegex),
		excludePodRegex:       compileRegex(cfg.ExcludePodRegex),
	}, nil
}

func (stp *sourceTraceProcessor) fillOtherMeta(atts pdata.AttributeMap) {
	atts.UpsertString("_source", "traces")
	if stp.collector != "" {
		atts.UpsertString("_collector", stp.collector)
	}
}

func (stp *sourceTraceProcessor) isFilteredOut(atts pdata.AttributeMap) bool {
	// TODO: This is quite inefficient when done for each package (ore even more so, span) separately.
	// It should be moved to K8S Meta Processor and done once per new pod/changed pod

	if value, found := atts.Get(excludeAnnotation); found {
		if value.Type() == pdata.AttributeValueSTRING && value.StringVal() == "true" {
			return true
		} else if value.Type() == pdata.AttributeValueBOOL && value.BoolVal() == true {
			return true
		}
	}

	if value, found := atts.Get(includeAnnotation); found {
		if value.Type() == pdata.AttributeValueSTRING && value.StringVal() == "true" {
			return false
		} else if value.Type() == pdata.AttributeValueBOOL && value.BoolVal() == true {
			return false
		}
	}

	if matchRegexMaybe(stp.excludeNamespaceRegex, atts, namespaceKey) {
		return true
	}

	if matchRegexMaybe(stp.excludePodRegex, atts, podKey) {
		return true
	}

	if matchRegexMaybe(stp.excludeContainerRegex, atts, containerKey) {
		return true
	}

	if matchRegexMaybe(stp.excludeHostRegex, atts, sourceHostKey) {
		return true
	}

	return false
}

func (stp *sourceTraceProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if rs.IsNil() {
			continue
		}
		res := rs.Resource()
		filledAnySource := false
		filledOtherMeta := false

		if !res.IsNil() {
			atts := res.Attributes()

			// TODO: move this to k8sprocessor
			enrichPodName(&atts)
			stp.fillOtherMeta(atts)
			filledOtherMeta = true

			filledAnySource = stp.sourceHostFiller.fillResourceOrUseAnnotation(&atts, sourceHostSpecialAnnotation) || filledAnySource
			filledAnySource = stp.sourceCategoryFiller.fillResourceOrUseAnnotation(&atts, sourceCategorySpecialAnnotation) || filledAnySource
			filledAnySource = stp.sourceNameFiller.fillResourceOrUseAnnotation(&atts, sourceNameSpecialAnnotation) || filledAnySource

			if stp.isFilteredOut(atts) {
				rs.InstrumentationLibrarySpans().Resize(0)
			}
		}

		// Perhaps this is coming through Zipkin and in such case the attributes are stored in each span attributes, doh!
		if !filledAnySource {
			ilss := rs.InstrumentationLibrarySpans()
			for j := 0; j < ilss.Len(); j++ {
				ils := ilss.At(j)
				if ils.IsNil() {
					continue
				}
				inputSpans := ils.Spans()
				outputSpans := pdata.NewSpanSlice()

				for k := 0; k < inputSpans.Len(); k++ {
					s := inputSpans.At(k)
					if s.IsNil() {
						continue
					}
					atts := s.Attributes()

					// TODO: move this to k8sprocessor
					enrichPodName(&atts)
					if !filledOtherMeta {
						stp.fillOtherMeta(atts)
					}

					stp.sourceHostFiller.fillResourceOrUseAnnotation(&atts, sourceHostSpecialAnnotation)
					stp.sourceCategoryFiller.fillResourceOrUseAnnotation(&atts, sourceCategorySpecialAnnotation)
					stp.sourceNameFiller.fillResourceOrUseAnnotation(&atts, sourceNameSpecialAnnotation)

					if !stp.isFilteredOut(atts) {
						outputSpans.Resize(outputSpans.Len() + 1)
						s.CopyTo(outputSpans.At(outputSpans.Len() - 1))
					}
				}

				ils.Spans().Resize(0)
				outputSpans.MoveAndAppendTo(ils.Spans())
			}
		}
	}
	return stp.nextConsumer.ConsumeTraces(ctx, td)
}

// GetCapabilities returns the Capabilities assocciated with the resource processor.
func (stp *sourceTraceProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}

// Start is invoked during service startup.
func (*sourceTraceProcessor) Start(_context context.Context, _host component.Host) error {
	return nil
}

// Shutdown is invoked during service shutdown.
func (*sourceTraceProcessor) Shutdown(_context context.Context) error {
	return nil
}

// Convert the pod_template_hash to an alphanumeric string using the same logic Kubernetes
// uses at https://github.com/kubernetes/apimachinery/blob/18a5ff3097b4b189511742e39151a153ee16988b/pkg/util/rand/rand.go#L119
func SafeEncodeString(s string) string {
	r := make([]byte, len(s))
	for i, b := range []rune(s) {
		r[i] = alphanums[(int(b) % len(alphanums))]
	}
	return string(r)
}

func enrichPodName(atts *pdata.AttributeMap) bool {
	// This replicates sanitize_pod_name function
	// Strip out dynamic bits from pod name.
	// NOTE: Kubernetes deployments append a template hash.
	// At the moment this can be in 3 different forms:
	//   1) pre-1.8: numeric in pod_template_hash and pod_parts[-2]
	//   2) 1.8-1.11: numeric in pod_template_hash, hash in pod_parts[-2]
	//   3) post-1.11: hash in pod_template_hash and pod_parts[-2]

	if atts == nil {
		return false
	}
	pod, found := atts.Get("pod")
	if !found {
		return false
	}

	podParts := strings.Split(pod.StringVal(), "-")
	if len(podParts) < 2 {
		// This is unexpected, fallback
		return false
	}

	podTemplateHashAttr, found := atts.Get(podTemplateHashKey)

	if found && len(podParts) > 2 {
		podTemplateHash := podTemplateHashAttr.StringVal()
		if podTemplateHash == podParts[len(podParts)-2] || SafeEncodeString(podTemplateHash) == podParts[len(podParts)-2] {
			atts.UpsertString(podNameKey, strings.Join(podParts[:len(podParts)-2], "-"))
			return true
		}
	}
	atts.UpsertString(podNameKey, strings.Join(podParts[:len(podParts)-1], "-"))
	return true
}

func extractFormat(format string, name string) attributeFiller {
	r, _ := regexp.Compile(`\%\{(\w+)\}`)

	labels := make([]string, 0)
	matches := r.FindAllStringSubmatch(format, -1)
	for _, matchset := range matches {
		labels = append(labels, matchset[1])
	}
	template := r.ReplaceAllString(format, "%s")

	return attributeFiller{
		name:            name,
		compiledFormat:  template,
		dashReplacement: "",
		labels:          labels,
		prefix:          "",
	}
}

func createSourceHostFiller() attributeFiller {
	return attributeFiller{
		name:            "_sourceHost",
		compiledFormat:  "",
		dashReplacement: "",
		labels:          make([]string, 0),
		prefix:          "",
	}
}

func createSourceNameFiller(cfg *Config) attributeFiller {
	filler := extractFormat(cfg.SourceName, "_sourceName")
	return filler
}

func createSourceCategoryFiller(cfg *Config) attributeFiller {
	filler := extractFormat(cfg.SourceCategory, "_sourceCategory")
	filler.compiledFormat = cfg.SourceCategoryPrefix + filler.compiledFormat
	filler.dashReplacement = cfg.SourceCategoryReplaceDash
	filler.prefix = cfg.SourceCategoryPrefix
	return filler
}

func (f *attributeFiller) fillResourceOrUseAnnotation(atts *pdata.AttributeMap, annotationKey string) bool {
	val, found := atts.Get(annotationKey)
	if found {
		annotationFiller := extractFormat(val.StringVal(), f.name)
		annotationFiller.dashReplacement = f.dashReplacement
		annotationFiller.compiledFormat = f.prefix + annotationFiller.compiledFormat
		return annotationFiller.fillAttributes(atts)
	}
	return f.fillAttributes(atts)
}

func (f *attributeFiller) fillAttributes(atts *pdata.AttributeMap) bool {
	if len(f.compiledFormat) == 0 {
		return false
	}

	labelValues := f.resourceLabelValues(atts)
	if labelValues != nil {
		str := fmt.Sprintf(f.compiledFormat, labelValues...)
		if f.dashReplacement != "" {
			str = strings.ReplaceAll(str, "-", f.dashReplacement)
		}
		atts.UpsertString(f.name, str)
		return true
	}
	return false
}

func (f *attributeFiller) resourceLabelValues(atts *pdata.AttributeMap) []interface{} {
	arr := make([]interface{}, 0)
	for _, label := range f.labels {
		value, ok := atts.Get(label)
		if !ok {
			return nil
		}
		arr = append(arr, value.StringVal())
	}
	return arr
}
