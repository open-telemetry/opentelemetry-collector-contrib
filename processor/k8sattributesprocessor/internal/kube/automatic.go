// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"regexp"
)

type AutomaticRules struct {
	Enabled            bool     `mapstructure:"enabled"`
	AnnotationPrefixes []string `mapstructure:"annotation_prefixes"`
}

const DefaultAnnotationPrefix = "resource.opentelemetry.io/"

func AutomaticAnnotationRule(prefix string) FieldExtractionRule {
	return FieldExtractionRule{
		Name:                 "$1",
		KeyRegex:             regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + `(.+)$`),
		HasKeyRegexReference: true,
		From:                 MetadataFromPod,
	}
}
