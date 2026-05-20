// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package url // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/url"

import (
	"fmt"

	"github.com/grafana/clusterurl/pkg/clusterurl"
)

type URLSanitizer struct {
	classifier *clusterurl.ClusterURLClassifier
	attributes map[string]bool
}

func NewURLSanitizer(config URLSanitizationConfig) (*URLSanitizer, error) {
	classifier, err := clusterurl.NewClusterURLClassifier(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create cluster URL classifier: %w", err)
	}

	attributes := make(map[string]bool)
	for _, attribute := range config.Attributes {
		attributes[attribute] = true
	}

	return &URLSanitizer{
		classifier: classifier,
		attributes: attributes,
	}, nil
}

func (s *URLSanitizer) SanitizeAttributeURL(url, attributeKey string) string {
	if url == "" {
		return url
	}

	if _, ok := s.attributes[attributeKey]; ok {
		return s.SanitizeURL(url)
	}

	return url
}

// SanitizeURL sanitizes the given URL by removing any gibberish words.
// https://github.com/open-telemetry/opentelemetry-ebpf-instrumentation/blob/38ca7938595409b8ffe6b897c14a0e3280dd2941/pkg/components/transform/route/cluster.go#L48
func (s *URLSanitizer) SanitizeURL(url string) string {
	return s.classifier.ClusterURL(url)
}
