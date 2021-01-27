// Copyright The OpenTelemetry Authors
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

package googlecloud

import (
	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// For more about monitored resources, see:
// https://cloud.google.com/logging/docs/api/v2/resource-list#resource-types

func getResource(e *entry.Entry) *mrpb.MonitoredResource {
	rt := detectResourceType(e)
	if rt == "" {
		return nil
	}

	switch rt {
	case "k8s_pod":
		return k8sPodResource(e)
	case "k8s_container":
		return k8sContainerResource(e)
	case "k8s_node":
		return k8sNodeResource(e)
	case "k8s_cluster":
		return k8sClusterResource(e)
	case "generic_node":
		return genericNodeResource(e)
	}

	return nil
}

func detectResourceType(e *entry.Entry) string {
	if hasResource("k8s.pod.name", e) {
		if hasResource("container.name", e) {
			return "k8s_container"
		}
		return "k8s_pod"
	}

	if hasResource("k8s.cluster.name", e) {
		if hasResource("host.name", e) {
			return "k8s_node"
		}
		return "k8s_cluster"
	}

	if hasResource("host.name", e) {
		return "generic_node"
	}

	return ""
}

func hasResource(key string, e *entry.Entry) bool {
	_, ok := e.Resource[key]
	return ok
}

func k8sPodResource(e *entry.Entry) *mrpb.MonitoredResource {
	return &mrpb.MonitoredResource{
		Type: "k8s_pod",
		Labels: map[string]string{
			"pod_name":       e.Resource["k8s.pod.name"],
			"namespace_name": e.Resource["k8s.namespace.name"],
			"cluster_name":   e.Resource["k8s.cluster.name"],
			// TODO project id
		},
	}
}

func k8sContainerResource(e *entry.Entry) *mrpb.MonitoredResource {
	return &mrpb.MonitoredResource{
		Type: "k8s_container",
		Labels: map[string]string{
			"container_name": e.Resource["container.name"],
			"pod_name":       e.Resource["k8s.pod.name"],
			"namespace_name": e.Resource["k8s.namespace.name"],
			"cluster_name":   e.Resource["k8s.cluster.name"],
			// TODO project id
		},
	}
}

func k8sNodeResource(e *entry.Entry) *mrpb.MonitoredResource {
	return &mrpb.MonitoredResource{
		Type: "k8s_node",
		Labels: map[string]string{
			"cluster_name": e.Resource["k8s.cluster.name"],
			"node_name":    e.Resource["host.name"],
			// TODO project id
		},
	}
}

func k8sClusterResource(e *entry.Entry) *mrpb.MonitoredResource {
	return &mrpb.MonitoredResource{
		Type: "k8s_cluster",
		Labels: map[string]string{
			"cluster_name": e.Resource["k8s.cluster.name"],
			// TODO project id
		},
	}
}

func genericNodeResource(e *entry.Entry) *mrpb.MonitoredResource {
	return &mrpb.MonitoredResource{
		Type: "generic_node",
		Labels: map[string]string{
			"node_id": e.Resource["host.name"],
			// TODO project id
		},
	}
}
