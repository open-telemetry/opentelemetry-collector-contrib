// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"

import (
	"fmt"
	"maps"
	"regexp"
)

type attributeRegexMatcher struct {
	pattern string
	re      *regexp.Regexp
}

func newAttributeRegexMatcher(key string, expectedValue any) (attributeRegexMatcher, error) {
	pattern, ok := expectedValue.(string)
	if !ok {
		return attributeRegexMatcher{}, fmt.Errorf("attribute %q/regex must be a string pattern", key)
	}
	re, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return attributeRegexMatcher{}, fmt.Errorf("attribute %q/regex has invalid pattern %q: %w", key, pattern, err)
	}
	return attributeRegexMatcher{pattern: pattern, re: re}, nil
}

func (m attributeRegexMatcher) validate(key string, actualValue any) error {
	actualStr, ok := actualValue.(string)
	if !ok {
		return fmt.Errorf("attribute %q must be a string to match /regex (got %T)", key, actualValue)
	}
	if !m.re.MatchString(actualStr) {
		return fmt.Errorf("attribute %q value %q does not match regex %q", key, actualStr, m.pattern)
	}
	return nil
}

type writeAttributeMatchers struct {
	exists map[string]struct{}
	regex  map[string]attributeRegexMatcher
}

func newWriteAttributeMatchers(opts writeOptions) (writeAttributeMatchers, error) {
	matchers := writeAttributeMatchers{
		exists: opts.attributeExists,
		regex:  make(map[string]attributeRegexMatcher, len(opts.attributeRegex)),
	}
	for key, pattern := range opts.attributeRegex {
		if _, ok := opts.attributeExists[key]; ok {
			return writeAttributeMatchers{}, fmt.Errorf("attribute %q cannot use both /exists and /regex", key)
		}
		matcher, err := newAttributeRegexMatcher(key, pattern)
		if err != nil {
			return writeAttributeMatchers{}, err
		}
		matchers.regex[key] = matcher
	}
	return matchers, nil
}

func applyWriteAttributeMatchers(doc *document, opts writeOptions) error {
	if len(opts.attributeExists) == 0 && len(opts.attributeRegex) == 0 {
		return nil
	}
	matchers, err := newWriteAttributeMatchers(opts)
	if err != nil {
		return err
	}
	for i := range doc.Resources {
		resource := &doc.Resources[i]
		if err := matchers.apply(resource.Attributes); err != nil {
			return fmt.Errorf("resource attributes: %w", err)
		}
		for j := range resource.Scopes {
			for k := range resource.Scopes[j].Metrics {
				metric := &resource.Scopes[j].Metrics[k]
				for l := range metric.Datapoints {
					if err := matchers.apply(metric.Datapoints[l].Attributes); err != nil {
						return fmt.Errorf("metric %q datapoint attributes: %w", metric.Name, err)
					}
				}
			}
		}
	}
	return nil
}

func (m writeAttributeMatchers) apply(attributes map[string]any) error {
	original := maps.Clone(attributes)
	for key := range m.exists {
		if _, ok := original[key]; !ok {
			continue
		}
		if err := replaceAttributeWithMatcher(attributes, key, key+"/exists", true); err != nil {
			return err
		}
	}
	for key, matcher := range m.regex {
		actualValue, ok := original[key]
		if !ok {
			continue
		}
		if err := matcher.validate(key, actualValue); err != nil {
			return err
		}
		if err := replaceAttributeWithMatcher(attributes, key, key+"/regex", matcher.pattern); err != nil {
			return err
		}
	}
	return nil
}

func replaceAttributeWithMatcher(attributes map[string]any, key, matcherKey string, matcherValue any) error {
	if _, ok := attributes[matcherKey]; ok {
		return fmt.Errorf("cannot generate matcher for attribute %q: attribute %q already exists", key, matcherKey)
	}
	delete(attributes, key)
	attributes[matcherKey] = matcherValue
	return nil
}
