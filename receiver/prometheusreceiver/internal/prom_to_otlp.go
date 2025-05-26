// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"net"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"
)

const removeOldSemconvFeatureGateID = "receiver.prometheusreceiver.RemoveLegacyResourceAttributes"

var removeOldSemconvFeatureGate = featuregate.GlobalRegistry().MustRegister(
	removeOldSemconvFeatureGateID,
	featuregate.StageBeta,
	featuregate.WithRegisterFromVersion("v0.101.0"),
	featuregate.WithRegisterDescription("When enabled, the net.host.name, net.host.port, and http.scheme resource attributes are no longer added to metrics. Use server.address, server.port, and url.scheme instead."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32814"),
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
	attrs.PutStr(string(conventions.ServiceNameKey), job)
	if isDiscernibleHost(host) {
		if !removeOldSemconvFeatureGate.IsEnabled() {
			attrs.PutStr(string(conventions.NetHostNameKey), host)
		}
		attrs.PutStr(string(conventions.ServerAddressKey), host)
	}
	attrs.PutStr(string(conventions.ServiceInstanceIDKey), instance)
	if !removeOldSemconvFeatureGate.IsEnabled() {
		attrs.PutStr(string(conventions.NetHostPortKey), port)
		attrs.PutStr(string(conventions.HTTPSchemeKey), serviceDiscoveryLabels.Get(model.SchemeLabel))
	}
	attrs.PutStr(string(conventions.ServerPortKey), port)
	attrs.PutStr(string(conventions.URLSchemeKey), serviceDiscoveryLabels.Get(model.SchemeLabel))

	addKubernetesResource(attrs, serviceDiscoveryLabels)

	return resource
}

// kubernetesDiscoveryToResourceAttributes maps from metadata labels discovered
// through the kubernetes implementation of service discovery to opentelemetry
// resource attribute keys.
var kubernetesDiscoveryToResourceAttributes = map[string]string{
	"__meta_kubernetes_pod_name":           string(conventions.K8SPodNameKey),
	"__meta_kubernetes_pod_uid":            string(conventions.K8SPodUIDKey),
	"__meta_kubernetes_pod_container_name": string(conventions.K8SContainerNameKey),
	"__meta_kubernetes_namespace":          string(conventions.K8SNamespaceNameKey),
	// Only one of the node name service discovery labels will be present
	"__meta_kubernetes_pod_node_name":      string(conventions.K8SNodeNameKey),
	"__meta_kubernetes_node_name":          string(conventions.K8SNodeNameKey),
	"__meta_kubernetes_endpoint_node_name": string(conventions.K8SNodeNameKey),
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
			attrs.PutStr(string(conventions.K8SReplicaSetNameKey), controllerName)
		case "DaemonSet":
			attrs.PutStr(string(conventions.K8SDaemonSetNameKey), controllerName)
		case "StatefulSet":
			attrs.PutStr(string(conventions.K8SStatefulSetNameKey), controllerName)
		case "Job":
			attrs.PutStr(string(conventions.K8SJobNameKey), controllerName)
		case "CronJob":
			attrs.PutStr(string(conventions.K8SCronJobNameKey), controllerName)
		}
	}
}
