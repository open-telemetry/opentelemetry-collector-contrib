// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// resolvedExtractRule is a FieldExtractConfig with its key regex, if any, compiled.
type resolvedExtractRule struct {
	tagName  string
	key      string
	keyRegex *regexp.Regexp
}

// compileKeyRegex compiles a key_regex so that it must match an argument key in full.
func compileKeyRegex(keyRegex string) (*regexp.Regexp, error) {
	return regexp.Compile("^(?:" + keyRegex + ")$")
}

// resolveExtractRules precompiles the regexes in a set of FieldExtractConfig rules.
func resolveExtractRules(rules []FieldExtractConfig) ([]resolvedExtractRule, error) {
	resolved := make([]resolvedExtractRule, 0, len(rules))
	for _, rule := range rules {
		r := resolvedExtractRule{tagName: rule.TagName, key: rule.Key}
		if rule.KeyRegex != "" {
			re, err := compileKeyRegex(rule.KeyRegex)
			if err != nil {
				return nil, fmt.Errorf("failed to compile key_regex %q: %w", rule.KeyRegex, err)
			}
			r.keyRegex = re
		}
		resolved = append(resolved, r)
	}
	return resolved, nil
}

// applyArgumentExtraction extracts values out of a queue's raw `arguments` map onto a
// resource, per the configured extraction rules. When a rule has no tag_name, the
// argument's own key is used as the attribute name. Attributes already present on the
// resource are never overwritten, so the first rule (and, within a key_regex rule, the
// lexically first matching key) to produce a given name wins.
func applyArgumentExtraction(rules []resolvedExtractRule, arguments map[string]any, resource pcommon.Resource) {
	if len(rules) == 0 || len(arguments) == 0 {
		return
	}

	attrs := resource.Attributes()
	putIfAbsent := func(name string, v any) {
		// Attribute names are only known at runtime, so they're set on the resource
		// directly instead of via the ResourceBuilder; guard against clobbering the
		// receiver's own attributes, e.g. tag_name: rabbitmq.queue.name.
		if _, exists := attrs.Get(name); !exists {
			attrs.PutStr(name, argumentValueString(v))
		}
	}

	// Map iteration order is random, so iterate keys in sorted order to keep the
	// result stable across scrapes when several keys map to the same attribute name.
	keys := slices.Sorted(maps.Keys(arguments))

	for _, rule := range rules {
		if rule.keyRegex == nil {
			if v, ok := arguments[rule.key]; ok {
				name := rule.tagName
				if name == "" {
					name = rule.key
				}
				putIfAbsent(name, v)
			}
			continue
		}

		for _, key := range keys {
			submatches := rule.keyRegex.FindStringSubmatchIndex(key)
			if submatches == nil {
				continue
			}

			name := rule.tagName
			if name == "" {
				name = key
			} else if rule.keyRegex.NumSubexp() > 0 {
				name = string(rule.keyRegex.ExpandString(nil, name, key, submatches))
			}
			putIfAbsent(name, arguments[key])
		}
	}
}

// argumentValueString renders a queue argument value as a string. JSON numbers decode
// as float64, so they're formatted explicitly to avoid exponent notation for large values.
func argumentValueString(v any) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}
