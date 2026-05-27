// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsattributelimitprocessor

import "strings"

// Tier constants define the drop priority.
// Lower tiers are dropped first. Tier 0 means the attribute is not droppable.
// Resource labels (node/pod) are dropped before scope and datapoint attrs,
// since scope and datapoint attrs are more likely to be needed for queries.
const (
	tierNotDroppable   = 0  // Protected attribute
	tier1HelmTooling   = 1  // Helm/tooling labels (node + pod)
	tier2K8sInternal   = 2  // K8s internal controller labels (node + pod)
	tier3VendorNode    = 3  // Vendor-specific node labels (nvidia, karpenter, aws.amazon.com)
	tier4EKSSystem     = 4  // EKS system labels (node only)
	tier5K8sSystemNode = 5  // K8s system node labels (kubernetes.io, topology, node.kubernetes.io, k8s.io)
	tier6CustomerNode  = 6  // Customer node labels (unknown prefix)
	tier7KnownPod      = 7  // Known vendor pod labels (batch.kubernetes.io, statefulset.kubernetes.io)
	tier8CustomerPod   = 8  // Customer pod labels (unknown prefix)
	tier9Scope         = 9  // Non-protected scope attributes (except cloudwatch.*)
	tier10Datapoint    = 10 // Non-protected datapoint attributes (last resort)
)

// protectedResourceKeys contains resource attribute keys that are never dropped.
var protectedResourceKeys = map[string]struct{}{
	// K8s identity
	"k8s.cluster.name":   {},
	"k8s.node.name":      {},
	"k8s.pod.name":       {},
	"k8s.pod.uid":        {},
	"k8s.namespace.name": {},
	"k8s.container.name": {},
	// K8s workload
	"k8s.deployment.name":  {},
	"k8s.statefulset.name": {},
	"k8s.daemonset.name":   {},
	"k8s.replicaset.name":  {},
	"k8s.job.name":         {},
	"k8s.cronjob.name":     {},
	"k8s.workload.name":    {},
	"k8s.workload.type":    {},
	// Device-specific
	"neurondevice":   {},
	"neuroncore":     {},
	"efa.device":     {},
	"aws.efa.eni.id": {},
	"volume_id":      {},
	"instance_id":    {},
	// App identity pod labels
	"k8s.pod.label.app.kubernetes.io/name":      {},
	"k8s.pod.label.app.kubernetes.io/instance":  {},
	"k8s.pod.label.app.kubernetes.io/component": {},
	// Control plane
	"k8s.component.name": {},
}

// protectedDatapointKeys contains datapoint attribute keys that are never dropped.
var protectedDatapointKeys = map[string]struct{}{
	// node_exporter
	"cpu":        {},
	"mode":       {},
	"device":     {},
	"mountpoint": {},
	"fstype":     {},
	// cadvisor
	"interface": {},
	// neuron — identity dimensions (collapsing these merges all core/device series)
	"memory_location": {},
	"percentile":      {},
	"neuroncore":      {},
	"neurondevice":    {},
	// neuron — status dimensions
	"status_type": {},
	"error_type":  {},
	// control plane
	"verb":         {},
	"code":         {},
	"method":       {},
	"request_kind": {},
	"resource":     {},
}

// protectedPrefixes contains resource attribute prefixes that are never dropped.
var protectedPrefixes = []string{
	"cloud.",
	"host.",
	"hw.",
}

// protectedScopePrefix is the scope attribute prefix that is never dropped.
const protectedScopePrefix = "cloudwatch."

// isProtectedResource returns true if the key is a protected resource attribute.
func isProtectedResource(key string) bool {
	if _, ok := protectedResourceKeys[key]; ok {
		return true
	}
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// isProtectedDatapoint returns true if the key is a protected datapoint attribute.
func isProtectedDatapoint(key string) bool {
	_, ok := protectedDatapointKeys[key]
	return ok
}

// helmToolingSuffixes are Helm/tooling label suffixes (node + pod scope).
var helmToolingSuffixes = map[string]struct{}{
	"helm.sh/chart":                {},
	"app.kubernetes.io/managed-by": {},
	"app.kubernetes.io/version":    {},
	"app.kubernetes.io/part-of":    {},
	"chart":                        {},
	"release":                      {},
	"heritage":                     {},
}

// k8sInternalSuffixes are K8s internal controller label suffixes (node + pod scope).
var k8sInternalSuffixes = map[string]struct{}{
	"pod-template-generation":            {},
	"statefulset.kubernetes.io/pod-name": {},
	"batch.kubernetes.io/controller-uid": {},
}

// nodeOnlySystemSuffixes are exact node label suffixes classified as EKS/system tier (Tier 4)
// that don't fall under the eks.amazonaws.com/ prefix catch-all.
var nodeOnlySystemSuffixes = map[string]struct{}{
	"node.kubernetes.io/lifecycle": {},
}

// vendorNodeLabelPrefixes are vendor-specific node label prefixes (Tier 3).
var vendorNodeLabelPrefixes = []string{
	"karpenter.sh/",
	"karpenter.k8s.aws/",
	"nvidia.com/",
	"aws.amazon.com/",
}

// k8sSystemNodeLabelPrefixes are K8s system node label prefixes (Tier 5).
var k8sSystemNodeLabelPrefixes = []string{
	"kubernetes.io/",
	"node.kubernetes.io/",
	"topology.kubernetes.io/",
	"k8s.io/",
}

// knownPodLabelPrefixes are prefixes for known pod label classification.
var knownPodLabelPrefixes = []string{
	"app.kubernetes.io/",
	"batch.kubernetes.io/",
	"statefulset.kubernetes.io/",
}

const (
	nodeLabelPrefix = "k8s.node.label."
	podLabelPrefix  = "k8s.pod.label."
)

// classifyAttribute returns the tier for a droppable attribute,
// or 0 (tierNotDroppable) if the attribute is protected or not classifiable.
//
// For datapoint attributes, all non-protected keys are tier 10.
// For scope attributes, all non-protected keys (except instrumentation.cloudwatch.*) are tier 9.
// For resource attributes, only K8s label attributes (k8s.node.label.*, k8s.pod.label.*)
// are subject to tier-based dropping (tiers 1-8). Other resource attrs (e.g. service.name,
// telemetry.sdk.*) are treated as non-droppable and handled only by force-prune.
func classifyAttribute(key, attrSource string) int {
	if attrSource == attrSourceDatapoint {
		if isProtectedDatapoint(key) {
			return tierNotDroppable
		}
		return tier10Datapoint
	}

	if attrSource == attrSourceScope {
		if strings.HasPrefix(key, protectedScopePrefix) {
			return tierNotDroppable
		}
		return tier9Scope
	}

	// Resource attribute classification.
	if isProtectedResource(key) {
		return tierNotDroppable
	}

	isNodeLabel := strings.HasPrefix(key, nodeLabelPrefix)
	isPodLabel := strings.HasPrefix(key, podLabelPrefix)

	if !isNodeLabel && !isPodLabel {
		return tierNotDroppable
	}

	var suffix string
	if isNodeLabel {
		suffix = key[len(nodeLabelPrefix):]
	} else {
		suffix = key[len(podLabelPrefix):]
	}

	if _, ok := helmToolingSuffixes[suffix]; ok {
		return tier1HelmTooling
	}

	if _, ok := k8sInternalSuffixes[suffix]; ok {
		return tier2K8sInternal
	}

	if isNodeLabel {
		// Vendor-specific node labels (Tier 3).
		for _, prefix := range vendorNodeLabelPrefixes {
			if strings.HasPrefix(suffix, prefix) {
				return tier3VendorNode
			}
		}

		// EKS system node labels (Tier 4).
		if _, ok := nodeOnlySystemSuffixes[suffix]; ok {
			return tier4EKSSystem
		}
		if strings.HasPrefix(suffix, "eks.amazonaws.com/") {
			return tier4EKSSystem
		}

		// K8s system node labels (Tier 5).
		for _, prefix := range k8sSystemNodeLabelPrefixes {
			if strings.HasPrefix(suffix, prefix) {
				return tier5K8sSystemNode
			}
		}

		// Customer node labels (Tier 6).
		return tier6CustomerNode
	}

	// Pod labels.
	for _, prefix := range knownPodLabelPrefixes {
		if strings.HasPrefix(suffix, prefix) {
			return tier7KnownPod
		}
	}

	return tier8CustomerPod
}
