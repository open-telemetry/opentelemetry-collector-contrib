// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8seventsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver"

import (
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver/internal/kube"
)

// option represents a configuration option that can be passes.
// to the k8s-tagger
type option func(*k8seventsReceiver) error

// withExtractLabels allows specifying options to control extraction of event labels.
func withExtractLabels(labels ...FieldExtractConfig) option {
	return func(p *k8seventsReceiver) error {
		labels, err := ExtractFieldRules("labels", labels...)
		if err != nil {
			return err
		}
		p.rules.Labels = labels
		return nil
	}
}

// withExtractAnnotations allows specifying options to control extraction of event annotations tags.
func withExtractAnnotations(annotations ...FieldExtractConfig) option {
	return func(p *k8seventsReceiver) error {
		annotations, err := ExtractFieldRules("annotations", annotations...)
		if err != nil {
			return err
		}
		p.rules.Annotations = annotations
		return nil
	}
}

func ExtractFieldRules(fieldType string, fields ...FieldExtractConfig) ([]kube.FieldExtractionRule, error) {
	var rules []kube.FieldExtractionRule
	for _, a := range fields {
		name := a.TagName

		if name == "" && a.Key != "" {
			// name for KeyRegex case is set at extraction time/runtime, skipped here
			name = fmt.Sprintf("k8s.event.%v.%v", fieldType, a.Key)
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

		rules = append(rules, kube.FieldExtractionRule{
			Name: name, Key: a.Key, KeyRegex: keyRegex, HasKeyRegexReference: hasKeyRegexReference,
		})
	}
	return rules, nil
}
