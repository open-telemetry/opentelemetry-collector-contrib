// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"fmt"
	"os"
	"regexp"
	"time"

	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

const (
	filterOPEquals       = "equals"
	filterOPNotEquals    = "not-equals"
	filterOPExists       = "exists"
	filterOPDoesNotExist = "does-not-exist"
	metadataPodIP        = "k8s.pod.ip"
	metadataPodStartTime = "k8s.pod.start_time"
	specPodHostName      = "k8s.pod.hostname"
	// TODO: use k8s.cluster.uid, container.image.repo_digests
	// from semconv when available,
	//   replace clusterUID with conventions.AttributeK8SClusterUID
	//   replace containerRepoDigests with conventions.AttributeContainerImageRepoDigests
	clusterUID                = "k8s.cluster.uid"
	containerImageRepoDigests = "container.image.repo_digests"
	containerPorts            = "k8s.container.ports"
	containerCPURequest       = "k8s.container.cpu.request"
)

// option represents a configuration option that can be passes.
// to the k8s-tagger
type option func(*kubernetesprocessor) error

// withAPIConfig provides k8s API related configuration to the processor.
// It defaults the authentication method to in-cluster auth using service accounts.
func withAPIConfig(cfg k8sconfig.APIConfig) option {
	return func(p *kubernetesprocessor) error {
		p.apiConfig = cfg
		return p.apiConfig.Validate()
	}
}

// withPassthrough enables passthrough mode. In passthrough mode, the processor
// only detects and tags the pod IP and does not invoke any k8s APIs.
func withPassthrough() option {
	return func(p *kubernetesprocessor) error {
		p.passthroughMode = true
		return nil
	}
}

// enabledAttributes returns the list of resource attributes enabled by default.
func enabledAttributes() (attributes []string) {
	defaultConfig := metadata.DefaultResourceAttributesConfig()
	if defaultConfig.K8sClusterUID.Enabled {
		attributes = append(attributes, clusterUID)
	}
	if defaultConfig.ContainerID.Enabled {
		attributes = append(attributes, conventions.AttributeContainerID)
	}
	if defaultConfig.ContainerImageName.Enabled {
		attributes = append(attributes, conventions.AttributeContainerImageName)
	}
	if defaultConfig.ContainerImageRepoDigests.Enabled {
		attributes = append(attributes, containerImageRepoDigests)
	}
	if defaultConfig.ContainerImageTag.Enabled {
		attributes = append(attributes, conventions.AttributeContainerImageTag)
	}
	if defaultConfig.ContainerPorts.Enabled {
		attributes = append(attributes, containerPorts)
	}
	if defaultConfig.ContainerResourcesRequestsCPU.Enabled {
		attributes = append(attributes, containerCPURequest)
	}
	if defaultConfig.K8sContainerName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SContainerName)
	}
	if defaultConfig.K8sCronjobName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SCronJobName)
	}
	if defaultConfig.K8sDaemonsetName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SDaemonSetName)
	}
	if defaultConfig.K8sDaemonsetUID.Enabled {
		attributes = append(attributes, conventions.AttributeK8SDaemonSetUID)
	}
	if defaultConfig.K8sDeploymentName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SDeploymentName)
	}
	if defaultConfig.K8sDeploymentUID.Enabled {
		attributes = append(attributes, conventions.AttributeK8SDeploymentUID)
	}
	if defaultConfig.K8sJobName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SJobName)
	}
	if defaultConfig.K8sJobUID.Enabled {
		attributes = append(attributes, conventions.AttributeK8SJobUID)
	}
	if defaultConfig.K8sNamespaceName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SNamespaceName)
	}
	if defaultConfig.K8sNodeName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SNodeName)
	}
	if defaultConfig.K8sNodeUID.Enabled {
		attributes = append(attributes, conventions.AttributeK8SNodeUID)
	}
	if defaultConfig.K8sPodHostname.Enabled {
		attributes = append(attributes, specPodHostName)
	}
	if defaultConfig.K8sPodName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SPodName)
	}
	if defaultConfig.K8sPodStartTime.Enabled {
		attributes = append(attributes, metadataPodStartTime)
	}
	if defaultConfig.K8sPodUID.Enabled {
		attributes = append(attributes, conventions.AttributeK8SPodUID)
	}
	if defaultConfig.K8sPodIP.Enabled {
		attributes = append(attributes, metadataPodIP)
	}
	if defaultConfig.K8sReplicasetName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SReplicaSetName)
	}
	if defaultConfig.K8sReplicasetUID.Enabled {
		attributes = append(attributes, conventions.AttributeK8SReplicaSetUID)
	}
	if defaultConfig.K8sStatefulsetName.Enabled {
		attributes = append(attributes, conventions.AttributeK8SStatefulSetName)
	}
	if defaultConfig.K8sStatefulsetUID.Enabled {
		attributes = append(attributes, conventions.AttributeK8SStatefulSetUID)
	}
	return
}

// withExtractMetadata allows specifying options to control extraction of pod metadata.
// If no fields explicitly provided, the defaults are pulled from metadata.yaml.
func withExtractMetadata(fields ...string) option {
	return func(p *kubernetesprocessor) error {
		for _, field := range fields {
			switch field {
			case conventions.AttributeK8SNamespaceName:
				p.rules.Namespace = true
			case conventions.AttributeK8SPodName:
				p.rules.PodName = true
			case conventions.AttributeK8SPodUID:
				p.rules.PodUID = true
			case specPodHostName:
				p.rules.PodHostName = true
			case metadataPodStartTime:
				p.rules.StartTime = true
			case metadataPodIP:
				p.rules.PodIP = true
			case conventions.AttributeK8SDeploymentName:
				p.rules.DeploymentName = true
			case conventions.AttributeK8SDeploymentUID:
				p.rules.DeploymentUID = true
			case conventions.AttributeK8SReplicaSetName:
				p.rules.ReplicaSetName = true
			case conventions.AttributeK8SReplicaSetUID:
				p.rules.ReplicaSetID = true
			case conventions.AttributeK8SDaemonSetName:
				p.rules.DaemonSetName = true
			case conventions.AttributeK8SDaemonSetUID:
				p.rules.DaemonSetUID = true
			case conventions.AttributeK8SStatefulSetName:
				p.rules.StatefulSetName = true
			case conventions.AttributeK8SStatefulSetUID:
				p.rules.StatefulSetUID = true
			case conventions.AttributeK8SContainerName:
				p.rules.ContainerName = true
			case conventions.AttributeK8SJobName:
				p.rules.JobName = true
			case conventions.AttributeK8SJobUID:
				p.rules.JobUID = true
			case conventions.AttributeK8SCronJobName:
				p.rules.CronJobName = true
			case conventions.AttributeK8SNodeName:
				p.rules.Node = true
			case conventions.AttributeK8SNodeUID:
				p.rules.NodeUID = true
			case conventions.AttributeContainerID:
				p.rules.ContainerID = true
			case conventions.AttributeContainerImageName:
				p.rules.ContainerImageName = true
			case containerImageRepoDigests:
				p.rules.ContainerImageRepoDigests = true
			case conventions.AttributeContainerImageTag:
				p.rules.ContainerImageTag = true
			case clusterUID:
				p.rules.ClusterUID = true
			case containerPorts:
				p.rules.ContainerPorts = true
			case containerCPURequest:
				p.rules.ContainerCPURequest = true
			}
		}
		return nil
	}
}

// withExtractLabels allows specifying options to control extraction of pod labels.
func withExtractLabels(labels ...FieldExtractConfig) option {
	return func(p *kubernetesprocessor) error {
		labels, err := extractFieldRules("labels", labels...)
		if err != nil {
			return err
		}
		p.rules.Labels = labels
		return nil
	}
}

// withExtractAnnotations allows specifying options to control extraction of pod annotations tags.
func withExtractAnnotations(annotations ...FieldExtractConfig) option {
	return func(p *kubernetesprocessor) error {
		annotations, err := extractFieldRules("annotations", annotations...)
		if err != nil {
			return err
		}
		p.rules.Annotations = annotations
		return nil
	}
}

func extractFieldRules(fieldType string, fields ...FieldExtractConfig) ([]kube.FieldExtractionRule, error) {
	var rules []kube.FieldExtractionRule
	for _, a := range fields {
		name := a.TagName

		if a.From == "" {
			a.From = kube.MetadataFromPod
		}

		if name == "" && a.Key != "" {
			// name for KeyRegex case is set at extraction time/runtime, skipped here
			name = fmt.Sprintf("k8s.%v.%v.%v", a.From, fieldType, a.Key)
		}

		var keyRegex *regexp.Regexp
		var hasKeyRegexReference bool
		if a.KeyRegex != "" {
			var err error
			keyRegex, err = regexp.Compile("^(?:" + a.KeyRegex + ")$")
			if err != nil {
				return rules, err
			}

			if keyRegex.NumSubexp() > 0 {
				hasKeyRegexReference = true
			}
		}

		rules = append(rules, kube.FieldExtractionRule{
			Name: name, Key: a.Key, KeyRegex: keyRegex, HasKeyRegexReference: hasKeyRegexReference, From: a.From,
		})
	}
	return rules, nil
}

// withFilterNode allows specifying options to control filtering pods by a node/host.
func withFilterNode(node, nodeFromEnvVar string) option {
	return func(p *kubernetesprocessor) error {
		if nodeFromEnvVar != "" {
			p.filters.Node = os.Getenv(nodeFromEnvVar)
			return nil
		}
		p.filters.Node = node
		return nil
	}
}

// withFilterNamespace allows specifying options to control filtering pods by a namespace.
func withFilterNamespace(ns string) option {
	return func(p *kubernetesprocessor) error {
		p.filters.Namespace = ns
		return nil
	}
}

// withFilterLabels allows specifying options to control filtering pods by pod labels.
func withFilterLabels(filters ...FieldFilterConfig) option {
	return func(p *kubernetesprocessor) error {
		var labels []kube.LabelFilter
		for _, f := range filters {
			var op selection.Operator
			switch f.Op {
			case filterOPNotEquals:
				op = selection.NotEquals
			case filterOPExists:
				op = selection.Exists
			case filterOPDoesNotExist:
				op = selection.DoesNotExist
			default:
				op = selection.Equals
			}
			labels = append(labels, kube.LabelFilter{
				Key:   f.Key,
				Value: f.Value,
				Op:    op,
			})
		}
		p.filters.Labels = labels
		return nil
	}
}

// withFilterFields allows specifying options to control filtering pods by pod fields.
func withFilterFields(filters ...FieldFilterConfig) option {
	return func(p *kubernetesprocessor) error {
		var fields []kube.FieldFilter
		for _, f := range filters {
			var op selection.Operator
			switch f.Op {
			case filterOPNotEquals:
				op = selection.NotEquals
			default:
				op = selection.Equals
			}
			fields = append(fields, kube.FieldFilter{
				Key:   f.Key,
				Value: f.Value,
				Op:    op,
			})
		}
		p.filters.Fields = fields
		return nil
	}
}

// withExtractPodAssociations allows specifying options to associate pod metadata with incoming resource
func withExtractPodAssociations(podAssociations ...PodAssociationConfig) option {
	return func(p *kubernetesprocessor) error {
		associations := make([]kube.Association, 0, len(podAssociations))
		var assoc kube.Association
		for _, association := range podAssociations {
			assoc = kube.Association{
				Sources: []kube.AssociationSource{},
			}

			var name string

			for _, associationSource := range association.Sources {
				if associationSource.From == kube.ConnectionSource {
					name = ""
				} else {
					name = associationSource.Name
				}
				assoc.Sources = append(assoc.Sources, kube.AssociationSource{
					From: associationSource.From,
					Name: name,
				})
			}
			associations = append(associations, assoc)
		}
		p.podAssociations = associations
		return nil
	}
}

// withExcludes allows specifying pods to exclude
func withExcludes(podExclude ExcludeConfig) option {
	return func(p *kubernetesprocessor) error {
		ignoredNames := kube.Excludes{}
		names := podExclude.Pods

		if len(names) == 0 {
			names = []ExcludePodConfig{{Name: "jaeger-agent"}, {Name: "jaeger-collector"}}
		}
		for _, name := range names {
			ignoredNames.Pods = append(ignoredNames.Pods, kube.ExcludePods{Name: regexp.MustCompile(name.Name)})
		}
		p.podIgnore = ignoredNames
		return nil
	}
}

// withWaitForMetadata allows specifying whether to wait for pod metadata to be synced.
func withWaitForMetadata(wait bool) option {
	return func(p *kubernetesprocessor) error {
		p.waitForMetadata = wait
		return nil
	}
}

// withWaitForMetadataTimeout allows specifying the timeout for waiting for pod metadata to be synced.
func withWaitForMetadataTimeout(timeout time.Duration) option {
	return func(p *kubernetesprocessor) error {
		p.waitForMetadataTimeout = timeout
		return nil
	}
}
