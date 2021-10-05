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

package k8sattributesprocessor

import (
	"fmt"
	"os"
	"regexp"

	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/kube"
)

const (
	filterOPEquals       = "equals"
	filterOPNotEquals    = "not-equals"
	filterOPExists       = "exists"
	filterOPDoesNotExist = "does-not-exist"
	// Used for maintaining backward compatibility
	metdataNamespace   = "namespace"
	metadataPodName    = "podName"
	metadataPodUID     = "podUID"
	metadataStartTime  = "startTime"
	metadataDeployment = "deployment"
	metadataCluster    = "cluster"
	metadataNode       = "node"
	// Will be removed when new fields get merged to https://github.com/open-telemetry/opentelemetry-collector/blob/main/model/semconv/opentelemetry.go
	metadataPodStartTime = "k8s.pod.start_time"
)

// Option represents a configuration option that can be passes.
// to the k8s-tagger
type Option func(*kubernetesprocessor) error

// WithAPIConfig provides k8s API related configuration to the processor.
// It defaults the authentication method to in-cluster auth using service accounts.
func WithAPIConfig(cfg k8sconfig.APIConfig) Option {
	return func(p *kubernetesprocessor) error {
		p.apiConfig = cfg
		return p.apiConfig.Validate()
	}
}

// WithPassthrough enables passthrough mode. In passthrough mode, the processor
// only detects and tags the pod IP and does not invoke any k8s APIs.
func WithPassthrough() Option {
	return func(p *kubernetesprocessor) error {
		p.passthroughMode = true
		return nil
	}
}

// WithExtractMetadata allows specifying options to control extraction of pod metadata.
// If no fields explicitly provided, all metadata extracted by default.
func WithExtractMetadata(fields ...string) Option {
	return func(p *kubernetesprocessor) error {
		if len(fields) == 0 {
			fields = []string{
				conventions.AttributeK8SNamespaceName,
				conventions.AttributeK8SPodName,
				conventions.AttributeK8SPodUID,
				metadataPodStartTime,
				conventions.AttributeK8SDeploymentName,
				conventions.AttributeK8SClusterName,
				conventions.AttributeK8SNodeName,
				conventions.AttributeContainerID,
				conventions.AttributeContainerImageName,
				conventions.AttributeContainerImageTag,
			}
		}
		for _, field := range fields {
			switch field {
			// Old conventions handled by the cases metdataNamespace, metadataPodName, metadataPodUID,
			// metadataStartTime, metadataDeployment, metadataCluster, metadataNode are being supported for backward compatibility.
			// These will be removed when new conventions get merged to https://github.com/open-telemetry/opentelemetry-collector/blob/main/model/semconv/opentelemetry.go
			case metdataNamespace, conventions.AttributeK8SNamespaceName:
				p.rules.Namespace = true
			case metadataPodName, conventions.AttributeK8SPodName:
				p.rules.PodName = true
			case metadataPodUID, conventions.AttributeK8SPodUID:
				p.rules.PodUID = true
			case metadataStartTime, metadataPodStartTime:
				p.rules.StartTime = true
			case metadataDeployment, conventions.AttributeK8SDeploymentName:
				p.rules.Deployment = true
			case metadataCluster, conventions.AttributeK8SClusterName:
				p.rules.Cluster = true
			case metadataNode, conventions.AttributeK8SNodeName:
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

// WithExtractLabels allows specifying options to control extraction of pod labels.
func WithExtractLabels(labels ...FieldExtractConfig) Option {
	return func(p *kubernetesprocessor) error {
		labels, err := extractFieldRules("labels", labels...)
		if err != nil {
			return err
		}
		p.rules.Labels = labels
		return nil
	}
}

// WithExtractAnnotations allows specifying options to control extraction of pod annotations tags.
func WithExtractAnnotations(annotations ...FieldExtractConfig) Option {
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
	rules := []kube.FieldExtractionRule{}
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

		if name == "" {
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

		rules = append(rules, kube.FieldExtractionRule{
			Name: name, Key: a.Key, Regex: r, From: a.From,
		})
	}
	return rules, nil
}

// WithFilterNode allows specifying options to control filtering pods by a node/host.
func WithFilterNode(node, nodeFromEnvVar string) Option {
	return func(p *kubernetesprocessor) error {
		if nodeFromEnvVar != "" {
			p.filters.Node = os.Getenv(nodeFromEnvVar)
			return nil
		}
		p.filters.Node = node
		return nil
	}
}

// WithFilterNamespace allows specifying options to control filtering pods by a namespace.
func WithFilterNamespace(ns string) Option {
	return func(p *kubernetesprocessor) error {
		p.filters.Namespace = ns
		return nil
	}
}

// WithFilterLabels allows specifying options to control filtering pods by pod labels.
func WithFilterLabels(filters ...FieldFilterConfig) Option {
	return func(p *kubernetesprocessor) error {
		labels := []kube.FieldFilter{}
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

// WithFilterFields allows specifying options to control filtering pods by pod fields.
func WithFilterFields(filters ...FieldFilterConfig) Option {
	return func(p *kubernetesprocessor) error {
		fields := []kube.FieldFilter{}
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

// WithExtractPodAssociations allows specifying options to associate pod metadata with incoming resource
func WithExtractPodAssociations(podAssociations ...PodAssociationConfig) Option {
	return func(p *kubernetesprocessor) error {
		associations := make([]kube.Association, 0, len(podAssociations))
		for _, association := range podAssociations {
			associations = append(associations, kube.Association{
				From: association.From,
				Name: association.Name,
			})
		}
		p.podAssociations = associations
		return nil
	}
}

// WithExcludes allows specifying pods to exclude
func WithExcludes(podExclude ExcludeConfig) Option {
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
