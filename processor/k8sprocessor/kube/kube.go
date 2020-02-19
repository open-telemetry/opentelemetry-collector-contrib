// Copyright 2019 Omnition Authors
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

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

const (
	podNodeField            = "spec.nodeName"
	ignoreAnnotation string = "opentelemetry.io/k8s-processor/ignore"

	defaultTagDeploymentName = "k8s.deployment.name"
	defaultTagNamespaceName  = "k8s.namespace.name"
	defaultTagPodName        = "k8s.pod.name"
	defaultTagNodeName       = "k8s.node.name"
	defaultTagClusterName    = "k8s.cluster.name"
	defaultTagStartTime      = "k8s.pod.startTime"
	defaultTagHostName       = "k8s.pod.hostname"
	defaultTagOwnerTemplate  = "k8s.owner.%s"
)

var (
	// TODO: move these to config with default values
	podNameIgnorePatterns = []*regexp.Regexp{
		regexp.MustCompile(`jaeger-agent`),
		regexp.MustCompile(`jaeger-collector`),
	}
	podDeleteGracePeriod = time.Second * 120
	watchSyncPeriod      = time.Minute * 5
)

// Client defines the main interface that allows querying pods by metadata.
type Client interface {
	GetPodByIP(string) (*Pod, bool)
	Start()
	Stop()
}

// ClientProvider defines a func type that returns a new Client.
type ClientProvider func(*zap.Logger, ExtractionRules, Filters, APIClientsetProvider, InformerProvider) (Client, error)

// APIClientsetProvider APIClientsetProvider defines a func type that initializes and return a new kubernetes
// Clientset object.
type APIClientsetProvider func() (*kubernetes.Clientset, error)

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
	Node      string
	Namespace string
	Fields    []FieldFilter
	Labels    []FieldFilter
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
	Deployment  bool
	Namespace   bool
	PodName     bool
	NodeName    bool
	ClusterName bool
	StartTime   bool
	HostName    bool
	Owners      bool

	Tags        ExtractionFieldTags
	Annotations []FieldExtractionRule
	Labels      []FieldExtractionRule
}

// ExtractionFieldTags is used to describe selected exported key names for the extracted data
type ExtractionFieldTags struct {
	Deployment    string
	Namespace     string
	PodName       string
	NodeName      string
	ClusterName   string
	StartTime     string
	HostName      string
	OwnerTemplate string
}

// NewExtractionFieldTags builds a new instance of tags with default values
func NewExtractionFieldTags() ExtractionFieldTags {
	tags := ExtractionFieldTags{}
	tags.Deployment = defaultTagDeploymentName
	tags.Namespace = defaultTagNamespaceName
	tags.PodName = defaultTagPodName
	tags.NodeName = defaultTagNodeName
	tags.ClusterName = defaultTagClusterName
	tags.StartTime = defaultTagStartTime
	tags.HostName = defaultTagHostName
	tags.OwnerTemplate = defaultTagOwnerTemplate
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
