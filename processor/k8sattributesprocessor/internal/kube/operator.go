package kube

import (
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"k8s.io/api/core/v1"
	"strings"
)

var serviceNamePrecedence = []string{
	conventions.AttributeK8SDeploymentName,
	conventions.AttributeK8SReplicaSetName,
	conventions.AttributeK8SStatefulSetName,
	conventions.AttributeK8SDaemonSetName,
	conventions.AttributeK8SCronJobName,
	conventions.AttributeK8SJobName,
	conventions.AttributeK8SPodName,
}

func OperatorServiceName(containerName string, names map[string]string) string {
	for _, k := range serviceNamePrecedence {
		if v, ok := names[k]; ok {
			return v
		}
	}
	return containerName
}

func createServiceInstanceID(pod *v1.Pod, containerName string) string {
	resNames := []string{pod.Namespace, pod.Name, containerName}
	return strings.Join(resNames, ".")
}
