// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"fmt"
	"os"
	"regexp"
	"time"

	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

// Config defines configuration for k8s attributes processor.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	// Passthrough mode only annotates resources with the pod IP and
	// does not try to extract any other metadata. It does not need
	// access to the K8S cluster API. Agent/Collector must receive spans
	// directly from services to be able to correctly detect the pod IPs.
	Passthrough bool `mapstructure:"passthrough"`

	// Extract section allows specifying extraction rules to extract
	// data from k8s pod specs
	Extract ExtractConfig `mapstructure:"extract"`

	// Filter section allows specifying filters to filter
	// pods by labels, fields, namespaces, nodes, etc.
	Filter FilterConfig `mapstructure:"filter"`

	// Association section allows to define rules for tagging spans, metrics,
	// and logs with Pod metadata.
	Association []PodAssociationConfig `mapstructure:"pod_association"`

	// Exclude section allows to define names of pod that should be
	// ignored while tagging.
	Exclude ExcludeConfig `mapstructure:"exclude"`

	// WaitForMetadata is a flag that determines if the processor should wait k8s metadata to be synced when starting.
	WaitForMetadata bool `mapstructure:"wait_for_metadata"`

	// WaitForMetadataTimeout is the maximum time the processor will wait for the k8s metadata to be synced.
	WaitForMetadataTimeout time.Duration `mapstructure:"wait_for_metadata_timeout"`
}

func (cfg *Config) Validate() error {
	if err := cfg.APIConfig.Validate(); err != nil {
		return err
	}

	for _, assoc := range cfg.Association {
		if len(assoc.Sources) > kube.PodIdentifierMaxLength {
			return fmt.Errorf("too many association sources. limit is %v", kube.PodIdentifierMaxLength)
		}
	}

	for _, f := range append(cfg.Extract.Labels, cfg.Extract.Annotations...) {
		if f.Key != "" && f.KeyRegex != "" {
			return fmt.Errorf("Out of Key or KeyRegex only one option is expected to be configured at a time, currently Key:%s and KeyRegex:%s", f.Key, f.KeyRegex)
		}

		switch f.From {
		case "", kube.MetadataFromPod, kube.MetadataFromNamespace, kube.MetadataFromNode, kube.MetadataFromDeployment, kube.MetadataFromStatefulSet, kube.MetadataFromDaemonSet, kube.MetadataFromJob:
		default:
			return fmt.Errorf("%s is not a valid choice for From. Must be one of: pod, namespace, deployment, statefulset, daemonset, job, node", f.From)
		}

		if f.KeyRegex != "" {
			_, err := regexp.Compile("^(?:" + f.KeyRegex + ")$")
			if err != nil {
				return err
			}
		}
	}

	for _, field := range cfg.Extract.Metadata {
		switch field {
		case string(conventions.K8SNamespaceNameKey), string(conventions.K8SPodNameKey), string(conventions.K8SPodUIDKey),
			string(conventions.K8SPodHostnameKey), string(conventions.K8SPodStartTimeKey), string(conventions.K8SPodIPKey),
			string(conventions.K8SDeploymentNameKey), string(conventions.K8SDeploymentUIDKey),
			string(conventions.K8SReplicaSetNameKey), string(conventions.K8SReplicaSetUIDKey),
			string(conventions.K8SDaemonSetNameKey), string(conventions.K8SDaemonSetUIDKey),
			string(conventions.K8SStatefulSetNameKey), string(conventions.K8SStatefulSetUIDKey),
			string(conventions.K8SJobNameKey), string(conventions.K8SJobUIDKey),
			string(conventions.K8SCronJobNameKey), string(conventions.K8SCronJobUIDKey),
			string(conventions.K8SNodeNameKey), string(conventions.K8SNodeUIDKey),
			string(conventions.K8SContainerNameKey), string(conventions.ContainerIDKey),
			string(conventions.ContainerImageNameKey), containerImageTag,
			string(conventions.ServiceNamespaceKey), string(conventions.ServiceNameKey),
			string(conventions.ServiceVersionKey), string(conventions.ServiceInstanceIDKey),
			string(conventions.ContainerImageRepoDigestsKey), string(conventions.K8SClusterUIDKey):
		default:
			return fmt.Errorf("\"%s\" is not a supported metadata field", field)
		}
	}

	for _, f := range cfg.Filter.Labels {
		switch f.Op {
		case "", filterOPEquals, filterOPNotEquals, filterOPExists, filterOPDoesNotExist:
		default:
			return fmt.Errorf("'%s' is not a valid label filter operation for key=%s, value=%s", f.Op, f.Key, f.Value)
		}
	}

	for _, f := range cfg.Filter.Fields {
		switch f.Op {
		case "", filterOPEquals, filterOPNotEquals:
		default:
			return fmt.Errorf("'%s' is not a valid label filter operation for key=%s, value=%s", f.Op, f.Key, f.Value)
		}
	}

	return nil
}

// ExtractConfig section allows specifying extraction rules to extract
// data from k8s pod specs.
type ExtractConfig struct {
	// Metadata allows to extract pod/namespace/node metadata from a list of metadata fields.
	// The field accepts a list of strings.
	//
	// Metadata fields supported right now are,
	//   k8s.pod.name, k8s.pod.uid, k8s.deployment.name,
	//   k8s.node.name, k8s.namespace.name, k8s.pod.start_time,
	//   k8s.replicaset.name, k8s.replicaset.uid,
	//   k8s.daemonset.name, k8s.daemonset.uid,
	//   k8s.job.name, k8s.job.uid,
	//   k8s.cronjob.name, k8s.cronjob.uid,
	//   k8s.statefulset.name, k8s.statefulset.uid,
	//   k8s.container.name, container.id, container.image.name,
	//   container.image.tag, container.image.repo_digests
	//   k8s.cluster.uid
	//
	// Specifying anything other than these values will result in an error.
	// By default, the following fields are extracted and added to spans, metrics and logs as resource attributes:
	//  - k8s.pod.name
	//  - k8s.pod.uid
	//  - k8s.pod.start_time
	//  - k8s.namespace.name
	//  - k8s.node.name
	//  - k8s.deployment.name (if the pod is controlled by a deployment)
	//  - k8s.container.name (requires an additional attribute to be set: container.id)
	//  - container.image.name (requires one of the following additional attributes to be set: container.id or k8s.container.name)
	//  - container.image.tag (requires one of the following additional attributes to be set: container.id or k8s.container.name)
	Metadata []string `mapstructure:"metadata"`

	// Annotations allows extracting data from pod annotations and record it
	// as resource attributes.
	// It is a list of FieldExtractConfig type. See FieldExtractConfig
	// documentation for more details.
	Annotations []FieldExtractConfig `mapstructure:"annotations"`

	// Labels allows extracting data from pod labels and record it
	// as resource attributes.
	// It is a list of FieldExtractConfig type. See FieldExtractConfig
	// documentation for more details.
	Labels []FieldExtractConfig `mapstructure:"labels"`

	// OtelAnnotations extracts all pod annotations with the prefix "resource.opentelemetry.io" as resource attributes
	// E.g. "resource.opentelemetry.io/foo" becomes "foo"
	OtelAnnotations bool `mapstructure:"otel_annotations"`

	// DeploymentNameFromReplicaSet allows extracting deployment name from replicaset name by trimming pod template hash.
	// This will disable watching for replicaset resources.
	DeploymentNameFromReplicaSet bool `mapstructure:"deployment_name_from_replicaset"`
}

// FieldExtractConfig allows specifying an extraction rule to extract a resource attribute from pod (or namespace)
// annotations (or labels).
type FieldExtractConfig struct {
	// TagName represents the name of the resource attribute that will be added to logs, metrics or spans.
	// When not specified, a default tag name will be used of the format:
	//   - k8s.pod.annotations.<annotation key>  (or k8s.pod.annotation.<annotation key> when processor.k8sattributes.EmitV1K8sConventions is enabled)
	//   - k8s.pod.labels.<label key>  (or k8s.pod.label.<label key> when processor.k8sattributes.EmitV1K8sConventions is enabled)
	// For example, if tag_name is not specified and the key is git_sha,
	// then the attribute name will be `k8s.pod.annotations.git_sha` (or `k8s.pod.annotation.git_sha` with the feature gate).
	// When key_regex is present, tag_name supports back reference to both named capturing and positioned capturing.
	// For example, if your pod spec contains the following labels,
	//
	// app.kubernetes.io/component: mysql
	// app.kubernetes.io/version: 5.7.21
	//
	// and you'd like to add tags for all labels with prefix app.kubernetes.io/ and also trim the prefix,
	// then you can specify the following extraction rules:
	//
	// extract:
	//   labels:
	//     - tag_name: $$1
	//       key_regex: kubernetes.io/(.*)
	//
	// this will add the `component` and `version` tags to the spans or metrics.
	// When key_regex is present without tag_name, the default tag name format will be used for each matched key.
	// For example:
	//
	// extract:
	//   labels:
	//     - key_regex: environment\.(.*)
	//       from: pod
	//
	// If labels like "environment.prod" and "environment.dev" exist, they will be extracted as
	// k8s.pod.labels.environment.prod and k8s.pod.labels.environment.dev respectively.
	TagName string `mapstructure:"tag_name"`

	// Key represents the annotation (or label) name. This must exactly match an annotation (or label) name.
	Key string `mapstructure:"key"`
	// KeyRegex is a regular expression used to extract a Key that matches the regex.
	// Out of Key or KeyRegex, only one option is expected to be configured at a time.
	KeyRegex string `mapstructure:"key_regex"`

	// From represents the source of the labels/annotations.
	// Allowed values are "pod", "namespace", and "node". The default is pod.
	From string `mapstructure:"from"`
}

// FilterConfig section allows specifying filters to filter
// pods by labels, fields, namespaces, nodes, etc.
type FilterConfig struct {
	// Node represents a k8s node or host. If specified, any pods not running
	// on the specified node will be ignored by the tagger.
	Node string `mapstructure:"node"`

	// NodeFromEnv can be used to extract the node name from an environment
	// variable. The value must be the name of the environment variable.
	// This is useful when the node a Otel agent will run on cannot be
	// predicted. In such cases, the Kubernetes downward API can be used to
	// add the node name to each pod as an environment variable. K8s tagger
	// can then read this value and filter pods by it.
	//
	// For example, node name can be passed to each agent with the downward API as follows
	//
	// env:
	//   - name: K8S_NODE_NAME
	//     valueFrom:
	//       fieldRef:
	//         fieldPath: spec.nodeName
	//
	// Then the NodeFromEnv field can be set to `K8S_NODE_NAME` to filter all pods by the node that
	// the agent is running on.
	//
	// More on downward API here: https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/
	NodeFromEnvVar string `mapstructure:"node_from_env_var"`

	// Namespace filters all pods by the provided namespace. All other pods are ignored.
	Namespace string `mapstructure:"namespace"`

	// Fields allows to filter pods by generic k8s fields.
	// Only the following operations are supported:
	//    - equals
	//    - not-equals
	//
	// Check FieldFilterConfig for more details.
	Fields []FieldFilterConfig `mapstructure:"fields"`

	// Labels allows to filter pods by generic k8s pod labels.
	// Only the following operations are supported:
	//    - equals
	//    - not-equals
	//    - exists
	//    - not-exists
	//
	// Check FieldFilterConfig for more details.
	Labels []FieldFilterConfig `mapstructure:"labels"`
}

func (cfg *FilterConfig) Validate() error {
	if cfg.NodeFromEnvVar != "" {
		if _, ok := os.LookupEnv(cfg.NodeFromEnvVar); !ok {
			return fmt.Errorf("`node_from_env_var` is configured but envvar %q is not set", cfg.NodeFromEnvVar)
		}
	}
	return nil
}

// FieldFilterConfig allows specifying exactly one filter by a field.
// It can be used to represent a label or generic field filter.
type FieldFilterConfig struct {
	// Key represents the key or name of the field or labels that a filter
	// can apply on.
	Key string `mapstructure:"key"`

	// Value represents the value associated with the key that a filter
	// operation specified by the `Op` field applies on.
	Value string `mapstructure:"value"`

	// Op represents the filter operation to apply on the given
	// Key: Value pair. The following operations are supported
	//   equals, not-equals, exists, does-not-exist.
	Op string `mapstructure:"op"`
}

// PodAssociationConfig contain single rule how to associate Pod metadata
// with logs, spans and metrics
type PodAssociationConfig struct {
	// List of pod association sources which should be taken
	// to identify pod
	Sources []PodAssociationSourceConfig `mapstructure:"sources"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ExcludeConfig represent a list of Pods to exclude
type ExcludeConfig struct {
	Pods []ExcludePodConfig `mapstructure:"pods"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ExcludePodConfig represent a Pod name to ignore
type ExcludePodConfig struct {
	Name string `mapstructure:"name"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type PodAssociationSourceConfig struct {
	// From represents the source of the association.
	// Allowed values are "connection" and "resource_attribute".
	From string `mapstructure:"from"`

	// Name represents extracted key name.
	// e.g. ip, pod_uid, k8s.pod.ip
	Name string `mapstructure:"name"`

	// prevent unkeyed literal initialization
	_ struct{}
}
