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

package kube

import (
	"regexp"
	"time"

	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	podNodeField            = "spec.nodeName"
	ignoreAnnotation string = "opentelemetry.io/k8s-processor/ignore"

	defaultTagContainerID     = "k8s.container.id"
	defaultTagContainerImage  = "k8s.container.image"
	defaultTagContainerName   = "k8s.container.name"
	defaultTagDaemonSetName   = "k8s.daemonset.name"
	defaultTagHostName        = "k8s.pod.hostname"
	defaultTagNodeName        = "k8s.node.name"
	defaultTagPodUID          = "k8s.pod.id"
	defaultTagReplicaSetName  = "k8s.replicaset.name"
	defaultTagServiceName     = "k8s.service.name"
	defaultTagStatefulSetName = "k8s.statefulset.name"
	defaultTagStartTime       = "k8s.pod.startTime"
)

var (
	// TODO: move these to config with default values
	podNameIgnorePatterns = []*regexp.Regexp{
		regexp.MustCompile(`jaeger-agent`),
		regexp.MustCompile(`jaeger-collector`),
		regexp.MustCompile(`otel-collector`),
		regexp.MustCompile(`otel-agent`),
		regexp.MustCompile(`collection-sumologic-otelcol`),
	}
	defaultPodDeleteGracePeriod = time.Second * 120
	watchSyncPeriod             = time.Minute * 5
)

// Client defines the main interface that allows querying pods by metadata.
type Client interface {
	GetPodByIP(string) (*Pod, bool)
	Start()
	Stop()
}

// ClientProvider defines a func type that returns a new Client.
type ClientProvider func(*zap.Logger, k8sconfig.APIConfig, ExtractionRules, Filters, APIClientsetProvider, InformerProvider, OwnerProvider) (Client, error)

// APIClientsetProvider defines a func type that initializes and return a new kubernetes
// Clientset object.
type APIClientsetProvider func(config k8sconfig.APIConfig) (kubernetes.Interface, error)

// Pod represents a kubernetes pod.
type Pod struct {
	Name       string
	Address    string
	Attributes map[string]string
	StartTime  *metav1.Time
	Ignore     bool

	DeletedAt time.Time
}

type deleteRequest struct {
	ip   string
	name string
	ts   time.Time
}

// Filters is used to instruct the client on how to filter out k8s pods.
// Right now only filters supported are the ones supported by k8s API itself
// for performance reasons. We can support adding additional custom filters
// in future if there is a real need.
type Filters struct {
	Node            string
	Namespace       string
	Fields          []FieldFilter
	Labels          []FieldFilter
	NamespaceLabels []FieldFilter
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

// ExtractionRules is used to specify the information that needs to be extracted
// from pods and added to the spans as tags.
type ExtractionRules struct {
	ClusterName     bool
	ContainerID     bool
	ContainerImage  bool
	ContainerName   bool
	DaemonSetName   bool
	DeploymentName  bool
	HostName        bool
	PodUID          bool
	PodName         bool
	ReplicaSetName  bool
	ServiceName     bool
	StatefulSetName bool
	StartTime       bool
	Namespace       bool
	NodeName        bool

	OwnerLookupEnabled bool

	Tags            ExtractionFieldTags
	Annotations     []FieldExtractionRule
	Labels          []FieldExtractionRule
	NamespaceLabels []FieldExtractionRule
}

// ExtractionFieldTags is used to describe selected exported key names for the extracted data
type ExtractionFieldTags struct {
	ClusterName     string
	ContainerID     string
	ContainerImage  string
	ContainerName   string
	DaemonSetName   string
	DeploymentName  string
	HostName        string
	PodUID          string
	PodName         string
	Namespace       string
	NodeName        string
	ReplicaSetName  string
	ServiceName     string
	StartTime       string
	StatefulSetName string
}

// NewExtractionFieldTags builds a new instance of tags with default values
func NewExtractionFieldTags() ExtractionFieldTags {
	tags := ExtractionFieldTags{}
	tags.ClusterName = conventions.AttributeK8sCluster
	tags.ContainerID = defaultTagContainerID
	tags.ContainerImage = defaultTagContainerImage
	tags.ContainerName = defaultTagContainerName
	tags.DaemonSetName = defaultTagDaemonSetName
	tags.DeploymentName = conventions.AttributeK8sDeployment
	tags.HostName = defaultTagHostName
	tags.PodUID = defaultTagPodUID
	tags.PodName = conventions.AttributeK8sPod
	tags.Namespace = conventions.AttributeK8sNamespace
	tags.NodeName = defaultTagNodeName
	tags.ReplicaSetName = defaultTagReplicaSetName
	tags.ServiceName = defaultTagServiceName
	tags.StartTime = defaultTagStartTime
	tags.StatefulSetName = defaultTagStatefulSetName
	return tags
}

// FieldExtractionRule is used to specify which fields to extract from pod fields
// and inject into spans as attributes.
type FieldExtractionRule struct {
	// Name is used to as the Span tag name.
	Name string
	// Key is used to lookup k8s pod fields.
	Key string
	// Regex is a regular expression used to extract a sub-part of a field value.
	// Full value is extracted when no regexp is provided.
	Regex *regexp.Regexp
}
