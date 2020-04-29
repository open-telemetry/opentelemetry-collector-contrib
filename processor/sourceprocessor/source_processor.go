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
	sourceHostSpecialAnnotation     string = "sumologic.com/sourceHost"
	sourceNameSpecialAnnotation     string = "sumologic.com/sourceName"
	sourceCategorySpecialAnnotation string = "sumologic.com/sourceCategory"
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
