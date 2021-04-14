// Copyright 2019 OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for Source processor.
type Config struct {
	*config.ProcessorSettings `mapstructure:"-"`

	Collector                 string `mapstructure:"collector"`
	Source                    string `mapstructure:"source"`
	SourceName                string `mapstructure:"source_name"`
	SourceCategory            string `mapstructure:"source_category"`
	SourceCategoryPrefix      string `mapstructure:"source_category_prefix"`
	SourceCategoryReplaceDash string `mapstructure:"source_category_replace_dash"`
	ExcludeNamespaceRegex     string `mapstructure:"exclude_namespace_regex"`
	ExcludePodRegex           string `mapstructure:"exclude_pod_regex"`
	ExcludeContainerRegex     string `mapstructure:"exclude_container_regex"`
	ExcludeHostRegex          string `mapstructure:"exclude_host_regex"`

	AnnotationPrefix   string `mapstructure:"annotation_prefix"`
	ContainerKey       string `mapstructure:"container_key"`
	NamespaceKey       string `mapstructure:"namespace_key"`
	PodKey             string `mapstructure:"pod_key"`
	PodIDKey           string `mapstructure:"pod_id_key"`
	PodNameKey         string `mapstructure:"pod_name_key"`
	PodTemplateHashKey string `mapstructure:"pod_template_hash_key"`
	SourceHostKey      string `mapstructure:"source_host_key"`
}
