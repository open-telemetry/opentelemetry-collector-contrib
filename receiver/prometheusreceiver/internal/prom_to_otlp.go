// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"net"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// isDiscernibleHost checks if a host can be used as a value for the 'host.name' key.
// localhost-like hosts and unspecified (0.0.0.0) hosts are not discernible.
func isDiscernibleHost(host string) bool {
	ip := net.ParseIP(host)
	if ip != nil {
		// An IP is discernible if
		//  - it's not local (e.g. belongs to 127.0.0.0/8 or ::1/128) and
		//  - it's not unspecified (e.g. the 0.0.0.0 address).
		return !ip.IsLoopback() && !ip.IsUnspecified()
	}

	if host == "localhost" {
		return false
	}

	// not an IP, not 'localhost', assume it is discernible.
	return true
}

// CreateResource creates the resource data added to OTLP payloads.
func CreateResource(job, instance string, serviceDiscoveryLabels labels.Labels) pcommon.Resource {
	host, port, err := net.SplitHostPort(instance)
	if err != nil {
		host = instance
	}
	resource := pcommon.NewResource()
	attrs := resource.Attributes()
	attrs.PutStr(conventions.AttributeServiceName, job)
	if isDiscernibleHost(host) {
		attrs.PutStr(conventions.AttributeNetHostName, host)
	}
	attrs.PutStr(conventions.AttributeServiceInstanceID, instance)
	attrs.PutStr(conventions.AttributeNetHostPort, port)
	attrs.PutStr(conventions.AttributeHTTPScheme, serviceDiscoveryLabels.Get(model.SchemeLabel))

	addKubernetesResource(attrs, serviceDiscoveryLabels)

	return resource
}

// kubernetesDiscoveryToResourceAttributes maps from metadata labels discovered
// through the kubernetes implementation of service discovery to opentelemetry
// resource attribute keys.
var kubernetesDiscoveryToResourceAttributes = map[string]string{
	"__meta_kubernetes_pod_name":           conventions.AttributeK8SPodName,
	"__meta_kubernetes_pod_uid":            conventions.AttributeK8SPodUID,
	"__meta_kubernetes_pod_container_name": conventions.AttributeK8SContainerName,
	"__meta_kubernetes_namespace":          conventions.AttributeK8SNamespaceName,
	// Only one of the node name service discovery labels will be present
	"__meta_kubernetes_pod_node_name":      conventions.AttributeK8SNodeName,
	"__meta_kubernetes_node_name":          conventions.AttributeK8SNodeName,
	"__meta_kubernetes_endpoint_node_name": conventions.AttributeK8SNodeName,
}

// addKubernetesResource adds resource information detected by prometheus'
// kubernetes service discovery.
func addKubernetesResource(attrs pcommon.Map, serviceDiscoveryLabels labels.Labels) {
	for sdKey, attributeKey := range kubernetesDiscoveryToResourceAttributes {
		if attr := serviceDiscoveryLabels.Get(sdKey); attr != "" {
			attrs.PutStr(attributeKey, attr)
		}
	}
	controllerName := serviceDiscoveryLabels.Get("__meta_kubernetes_pod_controller_name")
	controllerKind := serviceDiscoveryLabels.Get("__meta_kubernetes_pod_controller_kind")
	if controllerKind != "" && controllerName != "" {
		switch controllerKind {
		case "ReplicaSet":
			attrs.PutStr(conventions.AttributeK8SReplicaSetName, controllerName)
		case "DaemonSet":
			attrs.PutStr(conventions.AttributeK8SDaemonSetName, controllerName)
		case "StatefulSet":
			attrs.PutStr(conventions.AttributeK8SStatefulSetName, controllerName)
		case "Job":
			attrs.PutStr(conventions.AttributeK8SJobName, controllerName)
		case "CronJob":
			attrs.PutStr(conventions.AttributeK8SCronJobName, controllerName)
		}
	}
}
