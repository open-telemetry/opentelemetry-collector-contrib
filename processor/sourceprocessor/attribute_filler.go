// Copyright 2021 OpenTelemetry Authors
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
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
)

var (
	formatRegex *regexp.Regexp
)

func init() {
	var err error
	formatRegex, err = regexp.Compile(`\%\{(\w+)\}`)
	if err != nil {
		panic("failed to parse regex: " + err.Error())
	}
}

type attributeFiller struct {
	name            string
	compiledFormat  string
	dashReplacement string
	prefix          string
	labels          []string
}

func createSourceHostFiller() attributeFiller {
	return attributeFiller{
		name:            sourceHostKey,
		compiledFormat:  "",
		dashReplacement: "",
		labels:          make([]string, 0),
		prefix:          "",
	}
}

func extractFormat(format string, name string, keys sourceTraceKeys) attributeFiller {
	labels := make([]string, 0)
	matches := formatRegex.FindAllStringSubmatch(format, -1)
	for _, matchset := range matches {
		labels = append(labels, keys.convertKey(matchset[1]))
	}
	template := formatRegex.ReplaceAllString(format, "%s")

	return attributeFiller{
		name:            name,
		compiledFormat:  template,
		dashReplacement: "",
		labels:          labels,
		prefix:          "",
	}
}

func createSourceNameFiller(cfg *Config, keys sourceTraceKeys) attributeFiller {
	filler := extractFormat(cfg.SourceName, sourceNameKey, keys)
	return filler
}

func createSourceCategoryFiller(cfg *Config, keys sourceTraceKeys) attributeFiller {
	filler := extractFormat(cfg.SourceCategory, sourceCategoryKey, keys)
	filler.compiledFormat = cfg.SourceCategoryPrefix + filler.compiledFormat
	filler.dashReplacement = cfg.SourceCategoryReplaceDash
	filler.prefix = cfg.SourceCategoryPrefix
	return filler
}

func (f *attributeFiller) fillResourceOrUseAnnotation(atts *pdata.AttributeMap, annotationKey string, keys sourceTraceKeys) {
	val, found := atts.Get(annotationKey)
	if found {
		annotationFiller := extractFormat(val.StringVal(), f.name, keys)
		annotationFiller.dashReplacement = f.dashReplacement
		annotationFiller.compiledFormat = f.prefix + annotationFiller.compiledFormat
		annotationFiller.fillAttributes(atts)
	} else {
		f.fillAttributes(atts)
	}
}

func (f *attributeFiller) fillAttributes(atts *pdata.AttributeMap) {
	if len(f.compiledFormat) == 0 {
		return
	}

	labelValues := f.resourceLabelValues(atts)
	if labelValues != nil {
		str := fmt.Sprintf(f.compiledFormat, labelValues...)
		if f.dashReplacement != "" {
			str = strings.ReplaceAll(str, "-", f.dashReplacement)
		}
		atts.UpsertString(f.name, str)
	}
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
