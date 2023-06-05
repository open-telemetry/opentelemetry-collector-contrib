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

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"fmt"
	"regexp"
)

type sourceFormats struct {
	name     sourceFormat
	host     sourceFormat
	category sourceFormat
}

type sourceFormat struct {
	matches  []string
	template string
}

const sourceRegex = `\%\{([\w\.]+)\}`

// newSourceFormat builds sourceFormat basing on the regex and given text.
// Regex is basing on the `sourceRegex` const
// For given example text: `%{cluster}/%{namespace}“, it sets:
//   - template to `%s/%s`, which can be used later by fmt.Sprintf
//   - matches as map of (attribute) keys ({"cluster", "namespace"}) which will
//     be used to put corresponding value into templates' `%s
func newSourceFormat(r *regexp.Regexp, text string) sourceFormat {
	matches := r.FindAllStringSubmatch(text, -1)
	template := r.ReplaceAllString(text, "%s")

	m := make([]string, len(matches))

	for i, match := range matches {
		m[i] = match[1]
	}

	return sourceFormat{
		matches:  m,
		template: template,
	}
}

// newSourceFormats returns sourceFormats for name, host and category based on cfg
func newSourceFormats(cfg *Config) sourceFormats {
	r := regexp.MustCompile(sourceRegex)

	return sourceFormats{
		category: newSourceFormat(r, cfg.SourceCategory),
		host:     newSourceFormat(r, cfg.SourceHost),
		name:     newSourceFormat(r, cfg.SourceName),
	}
}

// format converts sourceFormat to string.
// Takes fields and put into template (%s placeholders) in order defined by matches
func (s *sourceFormat) format(f fields) string {
	labels := make([]interface{}, 0, len(s.matches))

	for _, matchset := range s.matches {
		v, ok := f.orig.Get(matchset)
		if ok {
			labels = append(labels, v.AsString())
		} else {
			labels = append(labels, "")
		}
	}

	return fmt.Sprintf(s.template, labels...)
}

// isSet returns true if template is non-empty
func (s *sourceFormat) isSet() bool {
	return len(s.template) > 0
}
