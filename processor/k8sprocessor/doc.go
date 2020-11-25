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

// Package k8sprocessor allow automatic tagging of spans, metrics and logs with k8s metadata.
//
// The processor automatically discovers k8s resources (pods), extracts metadata from them and adds the
// extracted metadata to the relevant spans, metrics and logs. The processor use the kubernetes API to discover all pods
// running in a cluster, keeps a record of their IP addresses and interesting metadata. Upon receiving telemetry data,
// the processor looks for presence of well-known resource attributes which might contain IP address ("ip",
// "k8s.pod.ip" for logs, metrics or traces and "host.name" for metrics). If this field is not available, or it
// does not contain a valid IP address, the processor tries to identify the source IP address of the service
// that sent the telemetry data.
// If a match is found, the cached metadata is added to the data as resource attributes.
//
// RBAC
//
// TODO: mention the required RBAC rules.
//
// Config
//
// TODO: example config.
//
// Deployment scenarios
//
// The processor supports running both in agent and collector mode.
//
// As an agent
//
// When running as an agent, the processor detects IP addresses of pods sending spans, metrics or logs to the agent
// and uses this information to extract metadata from pods. When running as an agent, it is important to apply
// a discovery filter so that the processor only discovers pods from the same host that it is running on. Not using
// such a filter can result in unnecessary resource usage especially on very large clusters. Once the filter is applied,
// each processor will only query the k8s API for pods running on it's own node.
//
// Node filter can be applied by setting the `filter.node` config option to the name of a k8s node. While this works
// as expected, it cannot be used to automatically filter pods by the same node that the processor is running on in
// most cases as it is not know before hand which node a pod will be scheduled on. Luckily, kubernetes has a solution
// for this called the downward API. To automatically filter pods by the node the processor is running on, you'll need
// to complete the following steps:
//
// 1. Use the downward API to inject the node name as an environment variable.
// Add the following snippet under the pod env section of the OpenTelemetry container.
//
//    env:
//    - name: KUBE_NODE_NAME
//      valueFrom:
//  	  fieldRef:
//  	    apiVersion: v1
//  	    fieldPath: spec.nodeName
//
// This will inject a new environment variable to the OpenTelemetry container with the value as the
// name of the node the pod was scheduled to run on.
//
// 2. Set "filter.node_from_env_var" to the name of the environment variable holding the node name.
//
//    k8s_tagger:
//      filter:
//        node_from_env_var: KUBE_NODE_NAME # this should be same as the var name used in previous step
//
// This will restrict each OpenTelemetry agent to query pods running on the same node only dramatically reducing
// resource requirements for very large clusters.
//
// As a collector
//
// The processor can be deployed both as an agent or as a collector.
//
// When running as a collector, the processor cannot correctly detect the IP address of the pods generating
// the telemetry data without any of the well-known IP attributes, when it receives them
// from an agent instead of receiving them directly from the pods. To
// workaround this issue, agents deployed with the k8s_tagger processor can be configured to detect
// the IP addresses and forward them along with the telemetry data resources. Collector can then match this IP address
// with k8s pods and enrich the records with the metadata. In order to set this up, you'll need to complete the
// following steps:
//
// 1. Setup agents in passthrough mode
// Configure the agents' k8s_tagger processors to run in passthrough mode.
//
//    # k8s_tagger config for agent
//    k8s_tagger:
//      passthrough: true
//
// This will ensure that the agents detect the IP address as add it as an attribute to all telemetry resources.
// Agents will not make any k8s API calls, do any discovery of pods or extract any metadata.
//
// 2. Configure the collector as usual
// No special configuration changes are needed to be made on the collector. It'll automatically detect
// the IP address of spans, logs and metrics sent by the agents as well as directly by other services/pods.
//
//
// Caveats
//
// There are some edge-cases and scenarios where k8s_tagger will not work properly.
//
//
// Host networking mode
//
// The processor cannot correct identify pods running in the host network mode and
// enriching telemetry data generated by such pods is not supported at the moment, unless the attributes contain
// information about the source IP.
//
// As a sidecar
//
// The processor does not support detecting containers from the same pods when running
// as a sidecar. While this can be done, we think it is simpler to just use the kubernetes
// downward API to inject environment variables into the pods and directly use their values
// as tags.
package k8sprocessor
