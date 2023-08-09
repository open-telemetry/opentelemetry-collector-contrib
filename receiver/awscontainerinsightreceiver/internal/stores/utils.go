// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"encoding/json"
	"strings"
	"time"

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
	namespace := metric.GetTag(ci.K8sNamespace)
	podName := metric.GetTag(ci.K8sPodNameKey)
	return k8sutil.CreatePodKey(namespace, podName)
}

func createContainerKeyFromMetric(metric CIMetric) string {
	namespace := metric.GetTag(ci.K8sNamespace)
	podName := metric.GetTag(ci.K8sPodNameKey)
	containerName := metric.GetTag(ci.ContainerNamekey)
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
	}

	if len(sources) > 0 {
		sourcesInfo, err := json.Marshal(sources)
		if err != nil {
			return
		}
		metric.AddTag(ci.SourcesKey, string(sourcesInfo))
	}
}

func AddKubernetesInfo(metric CIMetric, kubernetesBlob map[string]interface{}) {
	needMoveToKubernetes := map[string]string{ci.ContainerNamekey: "container_name", ci.K8sPodNameKey: "pod_name",
		ci.PodIDKey: "pod_id"}

	needCopyToKubernetes := map[string]string{ci.K8sNamespace: "namespace_name", ci.TypeService: "service_name", ci.NodeNameKey: "host"}

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
		metric.AddTag(ci.Kubernetes, string(kubernetesInfo))
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
