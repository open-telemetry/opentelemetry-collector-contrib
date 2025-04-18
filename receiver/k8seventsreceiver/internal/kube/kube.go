// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/kube"

import (
	"fmt"
	"regexp"
)

// ExtractionRules is used to specify the information that needs to be extracted
// from event and added to the spans as tags.
type ExtractionRules struct {
	Annotations []FieldExtractionRule
	Labels      []FieldExtractionRule
}

// FieldExtractionRule is used to specify which fields to extract from event fields
// and inject into spans as attributes.
type FieldExtractionRule struct {
	// Name is used to as the Span tag name.
	Name string
	// Key is used to lookup k8s event fields.
	Key string
	// KeyRegex is a regular expression(full length match) used to extract a Key that matches the regex.
	KeyRegex             *regexp.Regexp
	HasKeyRegexReference bool
	// Regex is a regular expression used to extract a sub-part of a field value.
	// Full value is extracted when no regexp is provided.
	Regex *regexp.Regexp
}

func (r *FieldExtractionRule) ExtractTagsFromMetadata(metadata map[string]string, formatter string) map[string]string {
	tags := make(map[string]string)
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
				tags[name] = v
			}
		}
	} else if v, ok := metadata[r.Key]; ok {
		tags[r.Name] = r.extractField(v)
	}
	return tags
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
