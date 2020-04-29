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
	"regexp"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
)

type sourceTraceProcessor struct {
	sourceCategoryFiller attributeFiller
	sourceNameFiller     attributeFiller
	sourceHostFiller     attributeFiller
	nextConsumer         consumer.TraceConsumer
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

	annotationPrefix                = "annotation:"
	podTemplateHashKey              = "label:pod-template-hash"
	podNameKey                      = "pod_name"
	sourceHostSpecialAnnotation     = annotationPrefix + "sumologic.com/sourceHost"
	sourceNameSpecialAnnotation     = annotationPrefix + "sumologic.com/sourceName"
	sourceCategorySpecialAnnotation = annotationPrefix + "sumologic.com/sourceCategory"
)

func newSourceTraceProcessor(next consumer.TraceConsumer, cfg *Config) (*sourceTraceProcessor, error) {
	return &sourceTraceProcessor{
		nextConsumer:         next,
		sourceHostFiller:     createSourceHostFiller(),
		sourceCategoryFiller: createSourceCategoryFiller(cfg),
		sourceNameFiller:     createSourceNameFiller(cfg),
	}, nil
}

func (stp *sourceTraceProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if rs.IsNil() {
			continue
		}
		res := rs.Resource()
		if !res.IsNil() {
			enrichPodName(res)
			stp.sourceHostFiller.fillResourceOrUseAnnotation(&res, sourceHostSpecialAnnotation)
			stp.sourceCategoryFiller.fillResourceOrUseAnnotation(&res, sourceCategorySpecialAnnotation)
			stp.sourceNameFiller.fillResourceOrUseAnnotation(&res, sourceNameSpecialAnnotation)
		}

		// TODO: iterate over span attributes too
		//ilss := rss.At(i).InstrumentationLibrarySpans()
		//for j := 0; j < ilss.Len(); j++ {
		//	ils := ilss.At(j)
		//	if ils.IsNil() {
		//		continue
		//	}
		//	spans := ils.Spans()
		//	for k := 0; k < spans.Len(); k++ {
		//		s := spans.At(k)
		//		if s.IsNil() {
		//			continue
		//		}
		//	}
		//}
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

func enrichPodName(res pdata.Resource) bool {
	// This replicates sanitize_pod_name function
	// Strip out dynamic bits from pod name.
	// NOTE: Kubernetes deployments append a template hash.
	// At the moment this can be in 3 different forms:
	//   1) pre-1.8: numeric in pod_template_hash and pod_parts[-2]
	//   2) 1.8-1.11: numeric in pod_template_hash, hash in pod_parts[-2]
	//   3) post-1.11: hash in pod_template_hash and pod_parts[-2]

	if res.IsNil() {
		return false
	}
	pod, found := res.Attributes().Get("pod")
	if !found {
		return false
	}

	podParts := strings.Split(pod.StringVal(), "-")
	if len(podParts) < 2 {
		// This is unexpected, fallback
		return false
	}

	podTemplateHashAttr, found := res.Attributes().Get(podTemplateHashKey)

	if found && len(podParts) > 2 {
		podTemplateHash := podTemplateHashAttr.StringVal()
		if podTemplateHash == podParts[len(podParts)-2] || SafeEncodeString(podTemplateHash) == podParts[len(podParts)-2] {
			res.Attributes().UpsertString(podNameKey, strings.Join(podParts[:len(podParts)-2], "-"))
			return true
		}
	}
	res.Attributes().UpsertString(podNameKey, strings.Join(podParts[:len(podParts)-1], "-"))
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

func (f *attributeFiller) fillResourceOrUseAnnotation(input *pdata.Resource, annotationKey string) bool {
	val, found := input.Attributes().Get(annotationKey)
	if found {
		annotationFiller := extractFormat(val.StringVal(), f.name)
		annotationFiller.dashReplacement = f.dashReplacement
		annotationFiller.compiledFormat = f.prefix + annotationFiller.compiledFormat
		return annotationFiller.fillResource(input)
	}
	return f.fillResource(input)
}

func (f *attributeFiller) fillResource(input *pdata.Resource) bool {
	if len(f.compiledFormat) == 0 {
		return false
	}

	labelValues := f.resourceLabelValues(input)
	if labelValues != nil {
		str := fmt.Sprintf(f.compiledFormat, labelValues...)
		if f.dashReplacement != "" {
			str = strings.ReplaceAll(str, "-", f.dashReplacement)
		}
		input.Attributes().UpsertString(f.name, str)
		return true
	}
	return false
}

func (f *attributeFiller) resourceLabelValues(input *pdata.Resource) []interface{} {
	arr := make([]interface{}, 0)
	attrs := input.Attributes()
	for _, label := range f.labels {
		value, ok := attrs.Get(label)
		if !ok {
			return nil
		}
		arr = append(arr, value.StringVal())
	}
	return arr
}
