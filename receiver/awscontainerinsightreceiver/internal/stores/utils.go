// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sutil"
)

const (
	// kubeAllowedStringAlphaNums holds the characters allowed in replicaset names from as parent deployment
	// https://github.com/kubernetes/apimachinery/blob/master/pkg/util/rand/rand.go#L83
	kubeAllowedStringAlphaNums = "bcdfghjklmnpqrstvwxz2456789"
	cronJobAllowedString       = "0123456789"
)

func createPodKeyFromMetaData(pod *corev1.Pod) string {
	namespace := pod.Namespace
	podName := pod.Name
	return k8sutil.CreatePodKey(namespace, podName)
}

func createPodKeyFromMetric(metric CIMetric) string {
	namespace := metric.GetTag(ci.AttributeK8sNamespace)
	podName := metric.GetTag(ci.AttributeK8sPodName)
	return k8sutil.CreatePodKey(namespace, podName)
}

func createContainerKeyFromMetric(metric CIMetric) string {
	namespace := metric.GetTag(ci.AttributeK8sNamespace)
	podName := metric.GetTag(ci.AttributeK8sPodName)
	containerName := metric.GetTag(ci.AttributeContainerName)
	return k8sutil.CreateContainerKey(namespace, podName, containerName)
}

// get the deployment name by stripping the last dash following some rules
// return empty if it is not following the rule
func parseDeploymentFromReplicaSet(name string) string {
	lastDash := strings.LastIndexAny(name, "-")
	if lastDash == -1 {
		// No dash
		return ""
	}
	suffix := name[lastDash+1:]
	if len(suffix) < 3 {
		// Invalid suffix if it is less than 3
		return ""
	}

	if !stringInRuneset(suffix, kubeAllowedStringAlphaNums) {
		// Invalid suffix
		return ""
	}

	return name[:lastDash]
}

// get the cronJob name by stripping the last dash following some rules
// return empty if it is not following the rule
func parseCronJobFromJob(name string) string {
	lastDash := strings.LastIndexAny(name, "-")
	if lastDash == -1 {
		// No dash
		return ""
	}
	suffix := name[lastDash+1:]
	if len(suffix) != 10 {
		// Invalid suffix if it is not 10 rune
		return ""
	}

	if !stringInRuneset(suffix, cronJobAllowedString) {
		// Invalid suffix
		return ""
	}

	return name[:lastDash]
}

func stringInRuneset(name, subset string) bool {
	for _, r := range name {
		if !strings.ContainsRune(subset, r) {
			// Found an unexpected rune in suffix
			return false
		}
	}
	return true
}

func TagMetricSource(metric CIMetric) {
	metricType := metric.GetTag(ci.MetricType)
	if metricType == "" {
		return
	}

	var sources []string
	switch metricType {
	case ci.TypeNode:
		sources = append(sources, []string{"cadvisor", "/proc", "pod", "calculated"}...)
	case ci.TypeNodeFS:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case ci.TypeNodeNet:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case ci.TypeNodeDiskIO:
		sources = append(sources, []string{"cadvisor"}...)
	case ci.TypePod:
		sources = append(sources, []string{"cadvisor", "pod", "calculated"}...)
	case ci.TypePodNet:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case ci.TypeContainer:
		sources = append(sources, []string{"cadvisor", "pod", "calculated"}...)
	case ci.TypeContainerFS:
		sources = append(sources, []string{"cadvisor", "calculated"}...)
	case ci.TypeContainerDiskIO:
		sources = append(sources, []string{"cadvisor"}...)
	case ci.TypeGpuContainer:
		sources = append(sources, []string{"dcgm", "pod", "calculated"}...)
	}

	if len(sources) > 0 {
		sourcesInfo, err := json.Marshal(sources)
		if err != nil {
			return
		}
		metric.AddTag(ci.SourcesKey, string(sourcesInfo))
	}
}

func AddKubernetesInfo(metric CIMetric, kubernetesBlob map[string]any, retainContainerNameTag bool) {
	needMoveToKubernetes := map[string]string{ci.AttributeK8sPodName: "pod_name", ci.AttributePodID: "pod_id"}
	needCopyToKubernetes := map[string]string{ci.AttributeK8sNamespace: "namespace_name", ci.TypeService: "service_name", ci.NodeNameKey: "host"}

	if retainContainerNameTag {
		needCopyToKubernetes[ci.AttributeContainerName] = "container_name"
	} else {
		needMoveToKubernetes[ci.AttributeContainerName] = "container_name"
	}

	for k, v := range needMoveToKubernetes {
		if attVal := metric.GetTag(k); attVal != "" {
			kubernetesBlob[v] = attVal
			metric.RemoveTag(k)
		}
	}
	for k, v := range needCopyToKubernetes {
		if attVal := metric.GetTag(k); attVal != "" {
			kubernetesBlob[v] = attVal
		}
	}

	if len(kubernetesBlob) > 0 {
		kubernetesInfo, err := json.Marshal(kubernetesBlob)
		if err != nil {
			return
		}
		metric.AddTag(ci.AttributeKubernetes, string(kubernetesInfo))
	}
}

func refreshWithTimeout(parentContext context.Context, refresh func(), timeout time.Duration) {
	ctx, cancel := context.WithTimeout(parentContext, timeout)
	// spawn a goroutine to process the actual refresh
	go func(cancel func()) {
		refresh()
		cancel()
	}(cancel)
	// block until either refresh() has executed or the timeout expires
	<-ctx.Done()
	cancel()
}

type RawContainerInsightsMetric struct {
	// source of the metric for debugging merge conflict
	ContainerName string
	// key/value pairs that are typed and contain the metric (numerical) data
	Fields map[string]any
	// key/value string pairs that are used to identify the metrics
	Tags   map[string]string
	Logger *zap.Logger
}

var _ CIMetric = (*RawContainerInsightsMetric)(nil)

func NewRawContainerInsightsMetric(mType string, logger *zap.Logger) *RawContainerInsightsMetric {
	metric := &RawContainerInsightsMetric{
		Fields: make(map[string]any),
		Tags:   make(map[string]string),
		Logger: logger,
	}
	metric.Tags[ci.MetricType] = mType
	return metric
}

func NewRawContainerInsightsMetricWithData(mType string, fields map[string]any, tags map[string]string, logger *zap.Logger) *RawContainerInsightsMetric {
	metric := NewRawContainerInsightsMetric(mType, logger)
	metric.Fields = fields
	metric.Tags = tags
	return metric
}

func (c *RawContainerInsightsMetric) GetTags() map[string]string {
	return c.Tags
}

func (c *RawContainerInsightsMetric) GetFields() map[string]any {
	return c.Fields
}

func (c *RawContainerInsightsMetric) GetMetricType() string {
	return c.Tags[ci.MetricType]
}

func (c *RawContainerInsightsMetric) AddTags(tags map[string]string) {
	for k, v := range tags {
		c.Tags[k] = v
	}
}

func (c *RawContainerInsightsMetric) HasField(key string) bool {
	return c.Fields[key] != nil
}

func (c *RawContainerInsightsMetric) AddField(key string, val any) {
	c.Fields[key] = val
}

func (c *RawContainerInsightsMetric) GetField(key string) any {
	return c.Fields[key]
}

func (c *RawContainerInsightsMetric) HasTag(key string) bool {
	return c.Tags[key] != ""
}

func (c *RawContainerInsightsMetric) AddTag(key, val string) {
	c.Tags[key] = val
}

func (c *RawContainerInsightsMetric) GetTag(key string) string {
	return c.Tags[key]
}

func (c *RawContainerInsightsMetric) RemoveTag(key string) {
	delete(c.Tags, key)
}

func (c *RawContainerInsightsMetric) Merge(src *RawContainerInsightsMetric) {
	// If there is any conflict, keep the Fields with earlier timestamp
	for k, v := range src.Fields {
		if _, ok := c.Fields[k]; ok {
			c.Logger.Debug(fmt.Sprintf("metric being merged has conflict in fields, src: %v, dest: %v \n", *src, *c))
			c.Logger.Debug("metric being merged has conflict in fields", zap.String("src", src.ContainerName), zap.String("dest", c.ContainerName))
			if c.Tags[ci.Timestamp] < src.Tags[ci.Timestamp] {
				continue
			}
		}
		c.Fields[k] = v
	}
}
