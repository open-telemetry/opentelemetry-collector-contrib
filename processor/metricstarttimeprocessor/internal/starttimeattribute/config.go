// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starttimeattribute

// AttributesFilterConfig holds the user configuration for filtering the k8s informer that is used by the Adjuster
// currently, this only supports a subset of all filter types
type AttributesFilterConfig struct {
	Node      string        `mapstructure:"node"`
	Namespace string        `mapstructure:"namespace"`
	Labels    []LabelFilter `mapstructure:"labels"`
}

type LabelFilter struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
	Op    string `mapstructure:"op"`
}
