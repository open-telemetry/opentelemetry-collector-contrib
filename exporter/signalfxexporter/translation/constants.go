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

    # dimensions
    container.image.name: container_image
    k8s.pod.name: kubernetes_pod_name
    k8s.pod.uid: kubernetes_pod_uid
    k8s.node.uid: kubernetes_node_uid
    k8s.node.name: kubernetes_node
    k8s.namespace.name: kubernetes_namespace
    k8s.cluster.name: kubernetes_cluster

    # properties
    k8s.workload.kind: kubernetes_workload
    k8s.workload.name: kubernetes_workload_name
    daemonset: daemonSet
    daemonset_uid: daemonSet_uid
    replicaset: replicaSet
    replicaset_uid: replicaSet_uid
    cronjob: cronJob
    cronjob_uid: cronJob_uid

- action: rename_metrics
  mapping:

    # kubeletstats receiver metrics
    container.cpu.time: container_cpu_utilization
    container.memory.usage: container_memory_usage_bytes
    container.filesystem.usage: container_fs_usage_bytes
    container.filesystem.available: container_fs_available_bytes
    container.filesystem.capacity: container_fs_capacity_bytes

    # k8s cluster receiver metrics
    k8s/pod/phase: kubernetes.pod_phase
    k8s/deployment/available: kubernetes.deployment.available
    k8s/deployment/desired: kubernetes.deployment.desired
    k8s/container/cpu/limit: kubernetes.container_cpu_limit
    k8s/container/cpu/request: kubernetes.container_cpu_request
    k8s/container/memory/limit: kubernetes.container_memory_limit
    k8s/container/memory/request: kubernetes.container_memory_request
    k8s/container/ready: kubernetes.container_ready
    k8s/node/condition_out_of_disk: kubernetes.node_out_of_disk
    k8s/node/condition_ready: kubernetes.node_ready
    k8s/node/condition_p_i_d_pressure: kubernetes.node_p_i_d_pressure
    k8s/node/condition_memory_pressure: kubernetes.node_memory_pressure
    k8s/node/condition_network_unavailable: kubernetes.node_network_unavailable

# container network metrics
- action: split_metric
  metric_name: k8s.pod.network.io
  dimension_key: direction
  mapping: 
    receive: pod_network_receive_bytes_total
    transmit: pod_network_transmit_bytes_total
- action: split_metric
  metric_name: k8s.pod.network.errors
  dimension_key: direction
  mapping: 
    receive: pod_network_receive_errors_total
    transmit: pod_network_transmit_errors_total

- action: split_metric
  metric_name: system.cpu.time
  dimension_key: state
  mapping:
    idle: cpu.idle
    interrupt: cpu.interrupt
    system: cpu.system
    user: cpu.user
    steal: cpu.steal
    wait: cpu.wait
    softirq: cpu.softirq
    nice: cpu.nice
- action: multiply_float
  scale_factors_float:
    container_cpu_utilization: 100
    cpu.idle: 100
    cpu.interrupt: 100
    cpu.system: 100
    cpu.user: 100
    cpu.steal: 100
    cpu.wait: 100
    cpu.softirq: 100
    cpu.nice: 100
- action: convert_values
  types_mapping:
    container_cpu_utilization: int
    cpu.idle: int
    cpu.interrupt: int
    cpu.system: int
    cpu.user: int
    cpu.steal: int
    cpu.wait: int
    cpu.softirq: int
    cpu.nice: int
`
)
