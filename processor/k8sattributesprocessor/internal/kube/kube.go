// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	podNodeField            = "spec.nodeName"
	ignoreAnnotation string = "opentelemetry.io/k8s-processor/ignore"
	tagNodeName             = "k8s.node.name"
	tagStartTime            = "k8s.pod.start_time"
	tagHostName             = "k8s.pod.hostname"
	tagClusterUID           = "k8s.cluster.uid"
	// MetadataFromPod is used to specify to extract metadata/labels/annotations from pod
	MetadataFromPod = "pod"
	// MetadataFromNamespace is used to specify to extract metadata/labels/annotations from namespace
	MetadataFromNamespace = "namespace"
	// MetadataFromNode is used to specify to extract metadata/labels/annotations from node
	MetadataFromNode       = "node"
	PodIdentifierMaxLength = 4

	ResourceSource   = "resource_attribute"
	ConnectionSource = "connection"
	K8sIPLabelName   = "k8s.pod.ip"
)

// PodIdentifierAttribute represents AssociationSource with matching value for pod
type PodIdentifierAttribute struct {
	Source AssociationSource
	Value  string
}

// PodIdentifier is a custom type to represent Pod identification
type PodIdentifier [PodIdentifierMaxLength]PodIdentifierAttribute

// IsNotEmpty checks if PodIdentifier is empty or not
func (p *PodIdentifier) IsNotEmpty() bool {
	return p[0].Source.From != ""
}

// PodIdentifierAttributeFromSource builds PodIdentifierAttribute using AssociationSource and value
func PodIdentifierAttributeFromSource(source AssociationSource, value string) PodIdentifierAttribute {
	return PodIdentifierAttribute{
		Source: source,
		Value:  value,
	}
}

// PodIdentifierAttributeFromSource builds PodIdentifierAttribute for connection with given value
func PodIdentifierAttributeFromConnection(value string) PodIdentifierAttribute {
	return PodIdentifierAttributeFromSource(
		AssociationSource{
			From: ConnectionSource,
			Name: "",
		},
		value,
	)
}

// PodIdentifierAttributeFromSource builds PodIdentifierAttribute for given resource_attribute name and value
func PodIdentifierAttributeFromResourceAttribute(key string, value string) PodIdentifierAttribute {
	return PodIdentifierAttributeFromSource(
		AssociationSource{
			From: ResourceSource,
			Name: key,
		},
		value,
	)
}

var (
	// TODO: move these to config with default values
	defaultPodDeleteGracePeriod = time.Second * 120
	watchSyncPeriod             = time.Minute * 5
)

// Client defines the main interface that allows querying pods by metadata.
type Client interface {
	GetPod(PodIdentifier) (*Pod, bool)
	GetNamespace(string) (*Namespace, bool)
	GetNode(string) (*Node, bool)
	Start() error
	Stop()
}

// ClientProvider defines a func type that returns a new Client.
type ClientProvider func(component.TelemetrySettings, k8sconfig.APIConfig, ExtractionRules, Filters, []Association, Excludes, APIClientsetProvider, InformerProvider, InformerProviderNamespace, InformerProviderReplicaSet, bool, time.Duration) (Client, error)

// APIClientsetProvider defines a func type that initializes and return a new kubernetes
// Clientset object.
type APIClientsetProvider func(config k8sconfig.APIConfig) (kubernetes.Interface, error)

// Pod represents a kubernetes pod.
type Pod struct {
	Name        string
	Address     string
	PodUID      string
	Attributes  map[string]string
	StartTime   *metav1.Time
	Ignore      bool
	Namespace   string
	NodeName    string
	HostNetwork bool

	// Containers specifies all containers in this pod.
	Containers PodContainers

	DeletedAt time.Time
}

// PodContainers specifies a list of pod containers. It is not safe for concurrent use.
type PodContainers struct {
	// ByID specifies all containers in a pod by container ID.
	ByID map[string]*Container
	// ByName specifies all containers in a pod by container name (k8s.container.name).
	ByName map[string]*Container
}

// Container stores resource attributes for a specific container defined by k8s pod spec.
type Container struct {
	Name      string
	ImageName string
	ImageTag  string

	// Statuses is a map of container k8s.container.restart_count attribute to ContainerStatus struct.
	Statuses map[int]ContainerStatus
}

// ContainerStatus stores resource attributes for a particular container run defined by k8s pod status.
type ContainerStatus struct {
	ContainerID     string
	ImageRepoDigest string
}

// Namespace represents a kubernetes namespace.
type Namespace struct {
	Name         string
	NamespaceUID string
	Attributes   map[string]string
	StartTime    metav1.Time
	DeletedAt    time.Time
}

// Node represents a kubernetes node.
type Node struct {
	Name       string
	NodeUID    string
	Attributes map[string]string
}

type deleteRequest struct {
	// id is identifier (IP address or Pod UID) of pod to remove from pods map
	id PodIdentifier
	// name contains name of pod to remove from pods map
	podName string
	ts      time.Time
}

// Filters is used to instruct the client on how to filter out k8s pods.
// Right now only filters supported are the ones supported by k8s API itself
// for performance reasons. We can support adding additional custom filters
// in future if there is a real need.
type Filters struct {
	Node      string
	Namespace string
	Fields    []FieldFilter
	Labels    []LabelFilter
}

// FieldFilter represents exactly one filter by field rule.
type FieldFilter struct {
	// Key matches the field name.
	Key string
	// Value matches the field value.
	Value string
	// Op determines the matching operation.
	// Currently only two operations are supported,
	//  - Equals
	//  - NotEquals
	Op selection.Operator
}

// LabelFilter represents exactly one filter by label rule.
type LabelFilter struct {
	// Key matches the label name.
	Key string
	// Value matches the label value.
	Value string
	// Op determines the matching operation.
	// The following operations are supported,
	//	- Exists
	// 	- DoesNotExist
	//	- Equals
	//	- DoubleEquals
	//	- NotEquals
	//	- GreaterThan
	//	- LessThan
	Op selection.Operator
}

// ExtractionRules is used to specify the information that needs to be extracted
// from pods and added to the spans as tags.
type ExtractionRules struct {
	CronJobName               bool
	DeploymentName            bool
	DeploymentUID             bool
	DaemonSetUID              bool
	DaemonSetName             bool
	JobUID                    bool
	JobName                   bool
	Namespace                 bool
	PodName                   bool
	PodUID                    bool
	PodHostName               bool
	PodIP                     bool
	ReplicaSetID              bool
	ReplicaSetName            bool
	StatefulSetUID            bool
	StatefulSetName           bool
	Node                      bool
	NodeUID                   bool
	StartTime                 bool
	ContainerName             bool
	ContainerID               bool
	ContainerImageName        bool
	ContainerImageRepoDigests bool
	ContainerImageTag         bool
	ClusterUID                bool

	Annotations []FieldExtractionRule
	Labels      []FieldExtractionRule
}

// IncludesOwnerMetadata determines whether the ExtractionRules include metadata about Pod Owners
func (rules *ExtractionRules) IncludesOwnerMetadata() bool {
	rulesNeedingOwnerMetadata := []bool{
		rules.CronJobName,
		rules.DeploymentName,
		rules.DeploymentUID,
		rules.DaemonSetUID,
		rules.DaemonSetName,
		rules.JobName,
		rules.JobUID,
		rules.ReplicaSetID,
		rules.ReplicaSetName,
		rules.StatefulSetUID,
		rules.StatefulSetName,
	}
	for _, ruleEnabled := range rulesNeedingOwnerMetadata {
		if ruleEnabled {
			return true
		}
	}
	return false
}

// FieldExtractionRule is used to specify which fields to extract from pod fields
// and inject into spans as attributes.
type FieldExtractionRule struct {
	// Name is used to as the Span tag name.
	Name string
	// Key is used to lookup k8s pod fields.
	Key string
	// KeyRegex is a regular expression(full length match) used to extract a Key that matches the regex.
	KeyRegex             *regexp.Regexp
	HasKeyRegexReference bool
	// Regex is a regular expression used to extract a sub-part of a field value.
	// Full value is extracted when no regexp is provided.
	Regex *regexp.Regexp
	// From determines the kubernetes object the field should be retrieved from.
	// Currently only three values are supported,
	//  - pod
	//  - namespace
	//  - node
	From string
}

func (r *FieldExtractionRule) extractFromPodMetadata(metadata map[string]string, tags map[string]string, formatter string) {
	// By default if the From field is not set for labels and annotations we want to extract them from pod
	if r.From == MetadataFromPod || r.From == "" {
		r.extractFromMetadata(metadata, tags, formatter)
	}
}

func (r *FieldExtractionRule) extractFromNamespaceMetadata(metadata map[string]string, tags map[string]string, formatter string) {
	if r.From == MetadataFromNamespace {
		r.extractFromMetadata(metadata, tags, formatter)
	}
}

func (r *FieldExtractionRule) extractFromNodeMetadata(metadata map[string]string, tags map[string]string, formatter string) {
	if r.From == MetadataFromNode {
		r.extractFromMetadata(metadata, tags, formatter)
	}
}

func (r *FieldExtractionRule) extractFromMetadata(metadata map[string]string, tags map[string]string, formatter string) {
	if r.KeyRegex != nil {
		for k, v := range metadata {
			if r.KeyRegex.MatchString(k) && v != "" {
				var name string
				if r.HasKeyRegexReference {
					var result []byte
					name = string(r.KeyRegex.ExpandString(result, r.Name, k, r.KeyRegex.FindStringSubmatchIndex(k)))
				} else {
					name = fmt.Sprintf(formatter, k)
				}
				tags[name] = v
			}
		}
	} else if v, ok := metadata[r.Key]; ok {
		tags[r.Name] = r.extractField(v)
	}
}

func (r *FieldExtractionRule) extractField(v string) string {
	// Check if a subset of the field should be extracted with a regular expression
	// instead of the whole field.
	if r.Regex == nil {
		return v
	}

	matches := r.Regex.FindStringSubmatch(v)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

// Associations represent a list of rules for Pod metadata associations with resources
type Associations struct {
	Associations []Association
}

// Association represents one association rule
type Association struct {
	Name    string
	Sources []AssociationSource
}

// Excludes represent a list of Pods to ignore
type Excludes struct {
	Pods []ExcludePods
}

// ExcludePods represent a Pod name to ignore
type ExcludePods struct {
	Name *regexp.Regexp
}

type AssociationSource struct {
	From string
	Name string
}

// Deployment represents a kubernetes deployment.
type Deployment struct {
	Name string
	UID  string
}

// ReplicaSet represents a kubernetes replicaset.
type ReplicaSet struct {
	Name       string
	Namespace  string
	UID        string
	Deployment Deployment
}
