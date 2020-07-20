// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

const (
	// DefaultTranslationRulesYaml defines default translation rules that will be applied to metrics if
	// config.SendCompatibleMetrics set to true and config.TranslationRules not specified explicitly.
	// Keep it in YAML format to be able to easily copy and paste it in config if modifications needed.
	DefaultTranslationRulesYaml = `
translation_rules:

- action: rename_dimension_keys
  mapping:
    container.image.name: container_image
    k8s.pod.name: kubernetes_pod_name
    k8s.pod.uid: kubernetes_pod_uid
    k8s.node.uid: kubernetes_node_uid
    k8s.node.name: kubernetes_node
    k8s.namespace.name: kubernetes_namespace
`
)
