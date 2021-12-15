// Copyright 2020, OpenTelemetry Authors
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

package k8sobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver"

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// convertNodeToEndpoint converts a node instance into a k8s.node observer.Endpoint. It will determine the
// Target by the first address match of InternalIP, InternalDNS, HostName, ExternalIP, and ExternalDNS in that
// order.
func convertNodeToEndpoint(idNamespace string, node *v1.Node) observer.Endpoint {
	nodeID := observer.EndpointID(fmt.Sprintf("%s/%s-%s", idNamespace, node.Name, node.UID))

	var internalIP, internalDNS, hostname, externalIP, externalDNS string

	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case v1.NodeInternalIP:
			internalIP = addr.Address
		case v1.NodeInternalDNS:
			internalDNS = addr.Address
		case v1.NodeHostName:
			hostname = addr.Address
		case v1.NodeExternalIP:
			externalIP = addr.Address
		case v1.NodeExternalDNS:
			externalDNS = addr.Address
		}
	}

	var target string
	for _, candidate := range []string{internalIP, internalDNS, hostname, externalIP, externalDNS} {
		if candidate != "" {
			target = candidate
			break
		}
	}

	// These fields are cleared to prevent excessive endpoint churn/receiver cycling
	node.ResourceVersion = ""
	for i := range node.Status.Conditions {
		node.Status.Conditions[i].LastHeartbeatTime = metav1.Time{}
		node.Status.Conditions[i].LastTransitionTime = metav1.Time{}
	}

	var metadata, spec, status map[string]interface{}
	for _, item := range []struct {
		src interface{}
		tgt *map[string]interface{}
	}{
		{node.ObjectMeta, &metadata}, {node.Spec, &spec}, {node.Status, &status},
	} {
		var jsonMap map[string]interface{}
		if marshaled, err := json.Marshal(item.src); err == nil {
			if err := json.Unmarshal(marshaled, &jsonMap); err == nil {
				configMap := config.NewMap()
				for k, v := range jsonMap {
					configMap.Set(k, v)
				}
				*(item.tgt) = configMap.ToStringMap()
			}
		}
	}

	nodeDetails := observer.K8sNode{
		UID:                 string(node.UID),
		Annotations:         node.Annotations,
		Labels:              node.Labels,
		Name:                node.Name,
		InternalIP:          internalIP,
		InternalDNS:         internalDNS,
		Hostname:            hostname,
		ExternalIP:          externalIP,
		ExternalDNS:         externalDNS,
		KubeletEndpointPort: uint16(node.Status.DaemonEndpoints.KubeletEndpoint.Port),
		Metadata:            metadata,
		Spec:                spec,
		Status:              status,
	}

	return observer.Endpoint{
		ID:      nodeID,
		Target:  target,
		Details: &nodeDetails,
	}
}
