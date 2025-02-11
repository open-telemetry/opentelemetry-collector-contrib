// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

const (
	// kubeAllowedStringAlphaNums holds the characters allowed in replicaset names from as parent deployment
	// https://github.com/kubernetes/apimachinery/blob/master/pkg/util/rand/rand.go#L83
	kubeAllowedStringAlphaNums = "bcdfghjklmnpqrstvwxz2456789"
	splitRegexStr              = "\\.|-"
	KubeProxy                  = "kube-proxy"
	cronJobAllowedString       = "0123456789"
)

var (
	re = regexp.MustCompile(splitRegexStr)

	podPhaseMetricNames = map[corev1.PodPhase]string{
		corev1.PodPending:   "pod_status_pending",
		corev1.PodRunning:   "pod_status_running",
		corev1.PodSucceeded: "pod_status_succeeded",
		corev1.PodFailed:    "pod_status_failed",
	}

	podConditionMetricNames = map[corev1.PodConditionType]string{
		corev1.PodReady:     "pod_status_ready",
		corev1.PodScheduled: "pod_status_scheduled",
	}

	podConditionUnknownMetric = "pod_status_unknown"
)

func addPodStatusMetrics(field map[string]any, pod *k8sclient.PodInfo) {
	for _, metricName := range podPhaseMetricNames {
		field[metricName] = 0
	}

	statusMetricName, validStatus := podPhaseMetricNames[pod.Phase]
	if validStatus {
		field[statusMetricName] = 1
	}
}

func addPodConditionMetrics(field map[string]any, pod *k8sclient.PodInfo) {
	for _, metricName := range podConditionMetricNames {
		field[metricName] = 0
	}

	field[podConditionUnknownMetric] = 0

	for _, condition := range pod.Conditions {
		switch condition.Status {
		case corev1.ConditionTrue:
			if statusMetricName, ok := podConditionMetricNames[condition.Type]; ok {
				field[statusMetricName] = 1
			}
		case corev1.ConditionUnknown:
			if _, ok := podConditionMetricNames[condition.Type]; ok {
				field[podConditionUnknownMetric] = 1
			}
		}
	}
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

func getJobNamePrefix(podName string) string {
	return re.Split(podName, 2)[0]
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

func isHyperPodNode(instanceType string) bool {
	return strings.HasPrefix(instanceType, "ml.")
}

func isLabelSet(conditionType int8, nodeLabels map[k8sclient.Label]int8, labelKey k8sclient.Label) (int, bool) {
	count := 0
	nodeConditions, labelExists := nodeLabels[labelKey]
	if labelExists && nodeConditions == conditionType {
		count = 1
	}
	return count, labelExists
}
