// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"fmt"
	"os"
	"regexp"

	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

const (
	filterOPEquals       = "equals"
	filterOPNotEquals    = "not-equals"
	filterOPExists       = "exists"
	filterOPDoesNotExist = "does-not-exist"
	metadataPodStartTime = "k8s.pod.start_time"
	specPodHostName      = "k8s.pod.hostname"
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

// withExtractMetadata allows specifying options to control extraction of pod metadata.
// If no fields explicitly provided, all metadata extracted by default.
func withExtractMetadata(fields ...string) option {
	return func(p *kubernetesprocessor) error {
		if len(fields) == 0 {
			fields = []string{
				conventions.AttributeK8SNamespaceName,
				conventions.AttributeK8SPodName,
				conventions.AttributeK8SPodUID,
				metadataPodStartTime,
				conventions.AttributeK8SDeploymentName,
				conventions.AttributeK8SNodeName,
				conventions.AttributeContainerImageName,
				conventions.AttributeContainerImageTag,
			}
		}
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
			case conventions.AttributeContainerID:
				p.rules.ContainerID = true
			case conventions.AttributeContainerImageName:
				p.rules.ContainerImageName = true
			case conventions.AttributeContainerImageTag:
				p.rules.ContainerImageTag = true
			default:
				return fmt.Errorf("\"%s\" is not a supported metadata field", field)
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

		switch a.From {
		// By default if the From field is not set for labels and annotations we want to extract them from pod
		case "", kube.MetadataFromPod:
			a.From = kube.MetadataFromPod
		case kube.MetadataFromNamespace:
			a.From = kube.MetadataFromNamespace
		default:
			return rules, fmt.Errorf("%s is not a valid choice for From. Must be one of: pod, namespace", a.From)
		}

		if name == "" && a.Key != "" {
			// name for KeyRegex case is set at extraction time/runtime, skipped here
			if a.From == kube.MetadataFromPod {
				name = fmt.Sprintf("k8s.pod.%s.%s", fieldType, a.Key)
			} else if a.From == kube.MetadataFromNamespace {
				name = fmt.Sprintf("k8s.namespace.%s.%s", fieldType, a.Key)
			}
		}

		var r *regexp.Regexp
		if a.Regex != "" {
			var err error
			r, err = regexp.Compile(a.Regex)
			if err != nil {
				return rules, err
			}
			names := r.SubexpNames()
			if len(names) != 2 || names[1] != "value" {
				return rules, fmt.Errorf("regex must contain exactly one named submatch (value)")
			}
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
			Name: name, Key: a.Key, KeyRegex: keyRegex, HasKeyRegexReference: hasKeyRegexReference, Regex: r, From: a.From,
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
		var labels []kube.FieldFilter
		for _, f := range filters {
			if f.Op == "" {
				f.Op = filterOPEquals
			}

			var op selection.Operator
			switch f.Op {
			case filterOPEquals:
				op = selection.Equals
			case filterOPNotEquals:
				op = selection.NotEquals
			case filterOPExists:
				op = selection.Exists
			case filterOPDoesNotExist:
				op = selection.DoesNotExist
			default:
				return fmt.Errorf("'%s' is not a valid label filter operation for key=%s, value=%s", f.Op, f.Key, f.Value)
			}
			labels = append(labels, kube.FieldFilter{
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
			if f.Op == "" {
				f.Op = filterOPEquals
			}

			var op selection.Operator
			switch f.Op {
			case filterOPEquals:
				op = selection.Equals
			case filterOPNotEquals:
				op = selection.NotEquals
			default:
				return fmt.Errorf("'%s' is not a valid field filter operation for key=%s, value=%s", f.Op, f.Key, f.Value)
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
