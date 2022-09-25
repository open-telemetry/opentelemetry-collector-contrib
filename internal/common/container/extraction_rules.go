// Copyright 2020 OpenTelemetry Authors
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

package container // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/container

import (
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// FieldExtractConfig allows specifying an extraction rule to extract a value from exactly one field.
//
// The field accepts a list FilterExtractConfig map. The map accepts several keys
//
//		tag_name, key, key_regex and regex
//
//	  - tag_name represents the name of the tag that will be added to the span.
//	    When not specified a default tag name will be used of the format:
//	    container.env_vars.<annotation key>
//	    container.labels.<label key>
//	    For example, if tag_name is not specified and the key is git_sha,
//	    then the attribute name will be `container.labels.git_sha`.
//	    When key_regex is present, tag_name supports back reference to both named capturing and positioned capturing.
//	    For example, if your container spec contains the following labels,
//
//	    app/component: mysql
//	    app/version: 5.7.21
//
//	    and you'd like to add tags for all labels with prefix app/ and also trim the prefix,
//	    then you can specify the following extraction rules:
//
//	    processors:
//	    docker_stats:
//	    extract:
//	    labels:
//
//	  - tag_name: $$1
//	    key_regex: app/(.*)
//
//	    this will add the `component` and `version` tags to the spans or metrics.
//
// - key represents the annotation name. This must exactly match an annotation name.
//
//   - regex is an optional field used to extract a sub-string from a complex field value.
//     The supplied regular expression must contain one named parameter with the string "value"
//     as the name. For example, if your container spec contains the following environment variable,
//
//     app/change-cause: 2019-08-28T18:34:33Z APP_NAME=my-app GIT_SHA=58a1e39 CI_BUILD=4120
//
//     and you'd like to extract the GIT_SHA and the CI_BUILD values as tags, then you must
//     specify the following two extraction rules:
//
//     processors:
//     docker_stats:
//     extract:
//     env_vars:
//
//   - tag_name: git.sha
//     key: app/change-cause
//     regex: GIT_SHA=(?P<value>\w+)
//
//   - tag_name: ci.build
//     key: app/change-cause
//     regex: JENKINS=(?P<value>[\w]+)
//
//     this will add the `git.sha` and `ci.build` tags to the spans or metrics.
type FieldExtractConfig struct {
	TagName string `mapstructure:"tag_name"`
	Key     string `mapstructure:"key"`
	// KeyRegex is a regular expression used to extract a Key that matches the regex.
	// Out of Key or KeyRegex only one option is expected to be configured at a time.
	KeyRegex string `mapstructure:"key_regex"`
	Regex    string `mapstructure:"regex"`
}

// FieldExtractionRule is used to specify which fields to extract from container fields
// and inject into spans as attributes.
type FieldExtractionRule struct {
	// Name is used to as the Span tag name.
	Name string
	// Key is used to lookup container fields.
	Key string
	// KeyRegex is a regular expression(full length match) used to extract a Key that matches the regex.
	KeyRegex             *regexp.Regexp
	HasKeyRegexReference bool
	// Regex is a regular expression used to extract a sub-part of a field value.
	// Full value is extracted when no regexp is provided.
	Regex *regexp.Regexp
}

// ExtractionRules is used to specify the information that needs to be extracted
// from containers and added to the spans as tags.
type ExtractionRules struct {
	EnvVars []FieldExtractionRule
	Labels  []FieldExtractionRule
}

func (r *FieldExtractionRule) ExtractFromMetadata(
	metadata map[string]string,
	tags pcommon.Map,
	formatter string,
) {
	if r.KeyRegex != nil {
		for k, v := range metadata {
			if r.KeyRegex.MatchString(k) && v != "" {
				var name string
				if r.HasKeyRegexReference {
					var result []byte
					name = string(r.KeyRegex.ExpandString(result, r.Name, k, r.KeyRegex.FindStringSubmatchIndex(k)))
				} else {
					name = fmt.Sprintf(formatter, k)
				}
				tags.PutString(name, v)
			}
		}
	} else if v, ok := metadata[r.Key]; ok {
		tags.PutString(r.Name, r.extractField(v))
	}
}

func (r *FieldExtractionRule) extractField(v string) string {
	// Check if a subset of the field should be extracted with a regular expression
	// instead of the whole field.
	if r.Regex == nil {
		return v
	}

	matches := r.Regex.FindStringSubmatch(v)
	if len(matches) == 2 {
		return matches[1]
	}

	return ""
}

func (c FieldExtractConfig) Validate() error {
	if c.Key != "" && c.KeyRegex != "" {
		return fmt.Errorf(
			"out of Key or KeyRegex only one option is expected to be configured at a time, currently Key:%s and KeyRegex:%s",
			c.Key, c.KeyRegex,
		)
	}

	return nil
}

func ExtractFieldRules(fieldType string, fields []FieldExtractConfig) ([]FieldExtractionRule, error) {
	var rules []FieldExtractionRule
	for _, a := range fields {
		name := a.TagName

		if name == "" && a.Key != "" {
			name = fmt.Sprintf("container.%s.%s", fieldType, a.Key)
		}

		var r *regexp.Regexp
		if a.Regex != "" {
			var err error
			r, err = regexp.Compile(a.Regex)
			if err != nil {
				return rules, err
			}
			names := r.SubexpNames()
			if len(names) != 2 || names[1] != "value" {
				return rules, fmt.Errorf("regex must contain exactly one named submatch (value)")
			}
		}

		var keyRegex *regexp.Regexp
		var hasKeyRegexReference bool
		if a.KeyRegex != "" {
			var err error
			keyRegex, err = regexp.Compile("^(?:" + a.KeyRegex + ")$")
			if err != nil {
				return rules, err
			}

			if keyRegex.NumSubexp() > 0 {
				hasKeyRegexReference = true
			}
		}

		rules = append(rules, FieldExtractionRule{
			Name: name, Key: a.Key, KeyRegex: keyRegex, HasKeyRegexReference: hasKeyRegexReference, Regex: r,
		})
	}

	return rules, nil
}
