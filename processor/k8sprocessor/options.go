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

package k8sprocessor

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/selection"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	filterOPEquals       = "equals"
	filterOPNotEquals    = "not-equals"
	filterOPExists       = "exists"
	filterOPDoesNotExist = "does-not-exist"

	metadataContainerID     = "containerId"
	metadataContainerName   = "containerName"
	metadataContainerImage  = "containerImage"
	metadataClusterName     = "clusterName"
	metadataDaemonSetName   = "daemonSetName"
	metadataDeploymentName  = "deploymentName"
	metadataHostName        = "hostName"
	metadataNamespace       = "namespace"
	metadataNodeName        = "nodeName"
	metadataPodID           = "podId"
	metadataPodName         = "podName"
	metadataReplicaSetName  = "replicaSetName"
	metadataServiceName     = "serviceName"
	metadataStartTime       = "startTime"
	metadataStatefulSetName = "statefulSetName"
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

// WithOwnerLookupEnabled makes the processor pull additional owner data from K8S API
func WithOwnerLookupEnabled() Option {
	return func(p *kubernetesprocessor) error {
		p.rules.OwnerLookupEnabled = true
		return nil
	}
}

// WithExtractMetadata allows specifying options to control extraction of pod metadata.
// If no fields explicitly provided, all metadata extracted by default.
func WithExtractMetadata(fields ...string) Option {
	return func(p *kubernetesprocessor) error {
		if len(fields) == 0 {
			fields = []string{
				metadataClusterName,
				metadataContainerID,
				metadataContainerImage,
				metadataContainerName,
				metadataDaemonSetName,
				metadataDeploymentName,
				metadataHostName,
				metadataNamespace,
				metadataNodeName,
				metadataPodName,
				metadataPodID,
				metadataReplicaSetName,
				metadataServiceName,
				metadataStartTime,
				metadataStatefulSetName,
			}
		}
		for _, field := range fields {
			switch field {
			case metadataClusterName:
				p.rules.ClusterName = true
			case metadataContainerID:
				p.rules.ContainerID = true
			case metadataContainerImage:
				p.rules.ContainerImage = true
			case metadataContainerName:
				p.rules.ContainerName = true
			case metadataDaemonSetName:
				p.rules.DaemonSetName = true
			case metadataDeploymentName:
				p.rules.DeploymentName = true
			case metadataHostName:
				p.rules.HostName = true
			case metadataNamespace:
				p.rules.Namespace = true
			case metadataNodeName:
				p.rules.NodeName = true
			case metadataPodID:
				p.rules.PodUID = true
			case metadataPodName:
				p.rules.PodName = true
			case metadataReplicaSetName:
				p.rules.ReplicaSetName = true
			case metadataServiceName:
				p.rules.ServiceName = true
			case metadataStartTime:
				p.rules.StartTime = true
			case metadataStatefulSetName:
				p.rules.StatefulSetName = true
			default:
				return fmt.Errorf("\"%s\" is not a supported metadata field", field)
			}
		}
		return nil
	}
}

// WithExtractTags allows specifying custom tag names
func WithExtractTags(tagsMap map[string]string) Option {
	return func(p *kubernetesprocessor) error {
		var tags = kube.NewExtractionFieldTags()
		for field, tag := range tagsMap {
			switch field {
			case strings.ToLower(metadataClusterName):
				tags.ClusterName = tag
			case strings.ToLower(metadataContainerID):
				tags.ContainerID = tag
			case strings.ToLower(metadataContainerName):
				tags.ContainerName = tag
			case strings.ToLower(metadataContainerImage):
				tags.ContainerImage = tag
			case strings.ToLower(metadataDaemonSetName):
				tags.DaemonSetName = tag
			case strings.ToLower(metadataDeploymentName):
				tags.DeploymentName = tag
			case strings.ToLower(metadataHostName):
				tags.HostName = tag
			case strings.ToLower(metadataNamespace):
				tags.Namespace = tag
			case strings.ToLower(metadataNodeName):
				tags.NodeName = tag
			case strings.ToLower(metadataPodID):
				tags.PodUID = tag
			case strings.ToLower(metadataPodName):
				tags.PodName = tag
			case strings.ToLower(metadataReplicaSetName):
				tags.ReplicaSetName = tag
			case strings.ToLower(metadataServiceName):
				tags.ServiceName = tag
			case strings.ToLower(metadataStartTime):
				tags.StartTime = tag
			case strings.ToLower(metadataStatefulSetName):
				tags.StatefulSetName = tag
			default:
				return fmt.Errorf("\"%s\" is not a supported metadata field", field)
			}
		}
		p.rules.Tags = tags
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
		if name == "" {
			if a.Key == "*" {
				name = fmt.Sprintf("k8s.%s.%%s", fieldType)
			} else {
				name = fmt.Sprintf("k8s.%s.%s", fieldType, a.Key)
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
			Name: name, Key: a.Key, Regex: r,
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
