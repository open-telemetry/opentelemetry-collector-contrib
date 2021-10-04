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
	"log"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/sourceprocessor/observability"
)

type sourceTraceKeys struct {
	annotationPrefix   string
	containerKey       string
	namespaceKey       string
	podKey             string
	podIDKey           string
	podNameKey         string
	podTemplateHashKey string
	sourceHostKey      string
}

func (stk sourceTraceKeys) convertKey(key string) string {
	switch key {
	case "container":
		return stk.containerKey
	case "namespace":
		return stk.namespaceKey
	case "pod":
		return stk.podKey
	case "pod_id":
		return stk.podIDKey
	case "pod_name":
		return stk.podNameKey
	case "source_host":
		return stk.sourceHostKey
	default:
		return key
	}
}

type sourceTraceProcessor struct {
	collector             string
	source                string
	sourceCategoryFiller  attributeFiller
	sourceNameFiller      attributeFiller
	sourceHostFiller      attributeFiller
	excludeNamespaceRegex *regexp.Regexp
	excludePodRegex       *regexp.Regexp
	excludeContainerRegex *regexp.Regexp
	excludeHostRegex      *regexp.Regexp
	keys                  sourceTraceKeys
}

const (
	alphanums = "bcdfghjklmnpqrstvwxz2456789"

	sourceHostSpecialAnnotation     = "sumologic.com/sourceHost"
	sourceNameSpecialAnnotation     = "sumologic.com/sourceName"
	sourceCategorySpecialAnnotation = "sumologic.com/sourceCategory"

	includeAnnotation = "sumologic.com/include"
	excludeAnnotation = "sumologic.com/exclude"

	collectorKey      = "_collector"
	sourceCategoryKey = "_sourceCategory"
	sourceHostKey     = "_sourceHost"
	sourceNameKey     = "_sourceName"
)

func compileRegex(regex string) *regexp.Regexp {
	if regex == "" {
		return nil
	}

	re, err := regexp.Compile(regex)
	if err != nil {
		log.Fatalf("Cannot compile regular expression: %s Error: %v\n", regex, err)
	}

	return re
}

func matchRegexMaybe(re *regexp.Regexp, atts pdata.AttributeMap, attributeName string) bool {
	if re == nil {
		return false
	}

	if attrValue, found := atts.Get(attributeName); found {
		if attrValue.Type() == pdata.AttributeValueTypeString {
			return re.MatchString(attrValue.StringVal())
		}
	}

	return false
}

func newSourceTraceProcessor(cfg *Config) *sourceTraceProcessor {
	keys := sourceTraceKeys{
		annotationPrefix:   cfg.AnnotationPrefix,
		containerKey:       cfg.ContainerKey,
		namespaceKey:       cfg.NamespaceKey,
		podIDKey:           cfg.PodIDKey,
		podKey:             cfg.PodKey,
		podNameKey:         cfg.PodNameKey,
		podTemplateHashKey: cfg.PodTemplateHashKey,
		sourceHostKey:      cfg.SourceHostKey,
	}

	return &sourceTraceProcessor{
		collector:             cfg.Collector,
		keys:                  keys,
		source:                cfg.Source,
		sourceHostFiller:      createSourceHostFiller(),
		sourceCategoryFiller:  createSourceCategoryFiller(cfg, keys),
		sourceNameFiller:      createSourceNameFiller(cfg, keys),
		excludeNamespaceRegex: compileRegex(cfg.ExcludeNamespaceRegex),
		excludeHostRegex:      compileRegex(cfg.ExcludeHostRegex),
		excludeContainerRegex: compileRegex(cfg.ExcludeContainerRegex),
		excludePodRegex:       compileRegex(cfg.ExcludePodRegex),
	}
}

func (stp *sourceTraceProcessor) fillOtherMeta(atts pdata.AttributeMap) {
	if stp.collector != "" {
		atts.UpsertString(collectorKey, stp.collector)
	}
}

func (stp *sourceTraceProcessor) isFilteredOut(atts pdata.AttributeMap) bool {
	// TODO: This is quite inefficient when done for each package (ore even more so, span) separately.
	// It should be moved to K8S Meta Processor and done once per new pod/changed pod

	if value, found := atts.Get(stp.annotationAttribute(excludeAnnotation)); found {
		if value.Type() == pdata.AttributeValueTypeString && value.StringVal() == "true" {
			return true
		} else if value.Type() == pdata.AttributeValueTypeBool && value.BoolVal() {
			return true
		}
	}

	if value, found := atts.Get(stp.annotationAttribute(includeAnnotation)); found {
		if value.Type() == pdata.AttributeValueTypeString && value.StringVal() == "true" {
			return false
		} else if value.Type() == pdata.AttributeValueTypeBool && value.BoolVal() {
			return false
		}
	}

	if matchRegexMaybe(stp.excludeNamespaceRegex, atts, stp.keys.namespaceKey) {
		return true
	}
	if matchRegexMaybe(stp.excludePodRegex, atts, stp.keys.podKey) {
		return true
	}
	if matchRegexMaybe(stp.excludeContainerRegex, atts, stp.keys.containerKey) {
		return true
	}
	if matchRegexMaybe(stp.excludeHostRegex, atts, stp.keys.sourceHostKey) {
		return true
	}

	return false
}

func (stp *sourceTraceProcessor) annotationAttribute(annotationKey string) string {
	return stp.keys.annotationPrefix + annotationKey
}

// ProcessTraces processes traces
func (stp *sourceTraceProcessor) ProcessTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()

	for i := 0; i < rss.Len(); i++ {
		observability.RecordResourceSpansProcessed()

		rs := rss.At(i)
		res := stp.processResource(rs.Resource())
		atts := res.Attributes()

		ilss := rs.InstrumentationLibrarySpans()
		totalSpans := 0
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			totalSpans += ils.Spans().Len()
		}

		if stp.isFilteredOut(atts) {
			rs.InstrumentationLibrarySpans().RemoveIf(func(pdata.InstrumentationLibrarySpans) bool { return true })
			observability.RecordFilteredOutN(totalSpans)
		} else {
			observability.RecordFilteredInN(totalSpans)
		}
	}

	return td, nil
}

// processResource performs multiple actions on resource:
//   - enrich pod name, so it can be used in templates
//   - fills source attributes based on config or annotations
//   - set metadata (collector name)
func (stp *sourceTraceProcessor) processResource(res pdata.Resource) pdata.Resource {
	atts := res.Attributes()

	stp.enrichPodName(&atts)
	stp.fillOtherMeta(atts)

	stp.sourceHostFiller.fillResourceOrUseAnnotation(&atts,
		stp.annotationAttribute(sourceHostSpecialAnnotation),
		stp.keys,
	)
	stp.sourceCategoryFiller.fillResourceOrUseAnnotation(&atts,
		stp.annotationAttribute(sourceCategorySpecialAnnotation),
		stp.keys,
	)
	stp.sourceNameFiller.fillResourceOrUseAnnotation(&atts,
		stp.annotationAttribute(sourceNameSpecialAnnotation),
		stp.keys,
	)

	return res
}

// SafeEncodeString converts the pod_template_hash to an alphanumeric string using the same logic Kubernetes
// uses at https://github.com/kubernetes/apimachinery/blob/18a5ff3097b4b189511742e39151a153ee16988b/pkg/util/rand/rand.go#L119
func SafeEncodeString(s string) string {
	r := make([]byte, len(s))
	for i, b := range []rune(s) {
		r[i] = alphanums[(int(b) % len(alphanums))]
	}
	return string(r)
}

func (stp *sourceTraceProcessor) enrichPodName(atts *pdata.AttributeMap) {
	// This replicates sanitize_pod_name function
	// Strip out dynamic bits from pod name.
	// NOTE: Kubernetes deployments append a template hash.
	// At the moment this can be in 3 different forms:
	//   1) pre-1.8: numeric in pod_template_hash and pod_parts[-2]
	//   2) 1.8-1.11: numeric in pod_template_hash, hash in pod_parts[-2]
	//   3) post-1.11: hash in pod_template_hash and pod_parts[-2]

	if atts == nil {
		return
	}
	pod, found := atts.Get(stp.keys.podKey)
	if !found {
		return
	}

	podParts := strings.Split(pod.StringVal(), "-")
	if len(podParts) < 2 {
		// This is unexpected, fallback
		return
	}

	podTemplateHashAttr, found := atts.Get(stp.keys.podTemplateHashKey)

	if found && len(podParts) > 2 {
		podTemplateHash := podTemplateHashAttr.StringVal()
		if podTemplateHash == podParts[len(podParts)-2] || SafeEncodeString(podTemplateHash) == podParts[len(podParts)-2] {
			atts.UpsertString(stp.keys.podNameKey, strings.Join(podParts[:len(podParts)-2], "-"))
			return
		}
	}
	atts.UpsertString(stp.keys.podNameKey, strings.Join(podParts[:len(podParts)-1], "-"))
}
