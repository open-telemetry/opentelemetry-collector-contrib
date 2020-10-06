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

package k8sprocessor

import (
	"go.opentelemetry.io/collector/config/configmodels"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config defines configuration for k8s attributes processor.
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`

	k8sconfig.APIConfig `mapstructure:",squash"`

	// Passthrough mode only annotates resources with the pod IP and
	// does not try to extract any other metadata. It does not need
	// access to the K8S cluster API. Agent/Collector must receive spans
	// directly from services to be able to correctly detect the pod IPs.
	Passthrough bool `mapstructure:"passthrough"`

	// Extract section allows specifying extraction rules to extract
	// data from k8s pod specs
	Extract ExtractConfig `mapstructure:"extract"`

	// Filter section allows specifying filters to filter
	// pods by labels, fields, namespaces, nodes, etc.
	Filter FilterConfig `mapstructure:"filter"`
}

// ExtractConfig section allows specifying extraction rules to extract
// data from k8s pod specs.
type ExtractConfig struct {
	// Metadata allows to extract pod metadata from a list of metadata fields.
	// The field accepts a list of strings.
	//
	// Metadata fields supported right now are,
	//   namespace, podName, podUID, deployment, cluster, node and startTime
	//
	// Specifying anything other than these values will result in an error.
	// By default all of the fields are extracted and added to spans and metrics.
	Metadata []string `mapstructure:"metadata"`

	// Annotations allows extracting data from pod annotations and record it
	// as resource attributes.
	// It is a list of FieldExtractConfig type. See FieldExtractConfig
	// documentation for more details.
	Annotations []FieldExtractConfig `mapstructure:"annotations"`

	// Annotations allows extracting data from pod labels and record it
	// as resource attributes.
	// It is a list of FieldExtractConfig type. See FieldExtractConfig
	// documentation for more details.
	Labels []FieldExtractConfig `mapstructure:"labels"`
}

// FieldExtractConfig allows specifying an extraction rule to extract a value from exactly one field.
//
// The field accepts a list FilterExtractConfig map. The map accepts three keys
//     tag_name, key and regex
//
// - tag_name represents the name of the tag that will be added to the span.
//   When not specified a default tag name will be used of the format:
//       k8s.pod.annotations.<annotation key>
//       k8s.pod.labels.<label key>
//   For example, if tag_name is not specified and the key is git_sha,
//   then the attribute name will be `k8s.pod.annotations.git_sha`.
//
// - key represents the annotation name. This must exactly match an annotation name.
//
// - regex is an optional field used to extract a sub-string from a complex field value.
//   The supplied regular expression must contain one named parameter with the string "value"
//   as the name. For example, if your pod spec contains the following annotation,
//
//		kubernetes.io/change-cause: 2019-08-28T18:34:33Z APP_NAME=my-app GIT_SHA=58a1e39 CI_BUILD=4120
//
//   and you'd like to extract the GIT_SHA and the CI_BUILD values as tags, then you must
//   specify the following two extraction rules:
//
//   procesors:
//     k8s-tagger:
//       annotations:
//         - name: git.sha
//           key: kubernetes.io/change-cause
//           regex: GIT_SHA=(?P<value>\w+)
//         - name: ci.build
//	         key: kubernetes.io/change-cause
//           regex: JENKINS=(?P<value>[\w]+)
//
//   this will add the `git.sha` and `ci.build` tags to the spans or metrics.
type FieldExtractConfig struct {
	TagName string `mapstructure:"tag_name"`
	Key     string `mapstructure:"key"`
	Regex   string `mapstructure:"regex"`
}

// FilterConfig section allows specifying filters to filter
// pods by labels, fields, namespaces, nodes, etc.
type FilterConfig struct {
	// Node represents a k8s node or host. If specified, any pods not running
	// on the specified node will be ignored by the tagger.
	Node string `mapstructure:"node"`

	// NodeFromEnv can be used to extract the node name from an environment
	// variable. The value must be the name of the environment variable.
	// This is useful when the node a Otel agent will run on cannot be
	// predicted. In such cases, the Kubernetes downward API can be used to
	// add the node name to each pod as an environment variable. K8s tagger
	// can then read this value and filter pods by it.
	//
	// For example, node name can be passed to each agent with the downward API as follows
	//
	// env:
	//   - name: K8S_NODE_NAME
	//     valueFrom:
	//       fieldRef:
	//         fieldPath: spec.nodeName
	//
	// Then the NodeFromEnv field can be set to `K8S_NODE_NAME` to filter all pods by the node that
	// the agent is running on.
	//
	// More on downward API here: https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/
	NodeFromEnvVar string `mapstructure:"node_from_env_var"`

	// Namespace filters all pods by the provided namespace. All other pods are ignored.
	Namespace string `mapstructure:"namespace"`

	// Fields allows to filter pods by generic k8s fields.
	// Only the following operations are supported:
	//    - equals
	//    - not-equals
	//
	// Check FieldFilterConfig for more details.
	Fields []FieldFilterConfig `mapstructure:"fields"`

	// Labels allows to filter pods by generic k8s pod labels.
	// Only the following operations are supported:
	//    - equals
	//    - not-equals
	//    - exists
	//    - not-exists
	//
	// Check FieldFilterConfig for more details.
	Labels []FieldFilterConfig `mapstructure:"labels"`
}

// FieldFilterConfig allows specifying exactly one filter by a field.
// It can be used to represent a label or generic field filter.
type FieldFilterConfig struct {
	// Key represents the key or name of the field or labels that a filter
	// can apply on.
	Key string `mapstructure:"key"`

	// Value represents the value associated with the key that a filter
	// operation specified by the `Op` field applies on.
	Value string `mapstructure:"value"`

	// Op represents the filter operation to apply on the given
	// Key: Value pair. The following operations are supported
	//   equals, not-equals, exists, does-not-exist.
	Op string `mapstructure:"op"`
}
