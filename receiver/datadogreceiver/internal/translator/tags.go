// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.21.0"
)

// See:
// https://docs.datadoghq.com/opentelemetry/schema_semantics/semantic_mapping/
// https://github.com/DataDog/opentelemetry-mapping-go/blob/main/pkg/otlp/attributes/attributes.go
var datadogKnownResourceAttributes = map[string]string{
	"env":     semconv.AttributeDeploymentEnvironment,
	"service": semconv.AttributeServiceName,
	"version": semconv.AttributeServiceVersion,

	// Container-related attributes
	"container_id":   semconv.AttributeContainerID,
	"container_name": semconv.AttributeContainerName,
	"image_name":     semconv.AttributeContainerImageName,
	"image_tag":      semconv.AttributeContainerImageTag,
	"runtime":        semconv.AttributeContainerRuntime,

	// Cloud-related attributes
	"cloud_provider": semconv.AttributeCloudProvider,
	"region":         semconv.AttributeCloudRegion,
	"zone":           semconv.AttributeCloudAvailabilityZone,

	// ECS-related attributes
	"task_family":        semconv.AttributeAWSECSTaskFamily,
	"task_arn":           semconv.AttributeAWSECSTaskARN,
	"ecs_cluster_name":   semconv.AttributeAWSECSClusterARN,
	"task_version":       semconv.AttributeAWSECSTaskRevision,
	"ecs_container_name": semconv.AttributeAWSECSContainerARN,

	// K8-related attributes
	"kube_container_name": semconv.AttributeK8SContainerName,
	"kube_cluster_name":   semconv.AttributeK8SClusterName,
	"kube_deployment":     semconv.AttributeK8SDeploymentName,
	"kube_replica_set":    semconv.AttributeK8SReplicaSetName,
	"kube_stateful_set":   semconv.AttributeK8SStatefulSetName,
	"kube_daemon_set":     semconv.AttributeK8SDaemonSetName,
	"kube_job":            semconv.AttributeK8SJobName,
	"kube_cronjob":        semconv.AttributeK8SCronJobName,
	"kube_namespace":      semconv.AttributeK8SNamespaceName,
	"pod_name":            semconv.AttributeK8SPodName,

	// Other
	"process_id":       semconv.AttributeProcessPID,
	"error.stacktrace": semconv.AttributeExceptionStacktrace,
	"error.msg":        semconv.AttributeExceptionMessage,
}

// translateDatadogTagToKeyValuePair translates a Datadog tag to a key value pair
func translateDatadogTagToKeyValuePair(tag string) (key string, value string) {
	if tag == "" {
		return "", ""
	}

	key, val, ok := strings.Cut(tag, ":")
	if !ok {
		// Datadog allows for two tag formats, one of which includes a key such as 'env',
		// followed by a value. Datadog also supports inputTags without the key, but OTel seems
		// to only support key:value pairs.
		// The following is a workaround to map unnamed inputTags to key:value pairs and its subject to future
		// changes if OTel supports unnamed inputTags in the future or if there is a better way to do this.
		key = "unnamed_" + tag
		val = tag
	}
	return key, val
}

// translateDatadogKeyToOTel translates a Datadog key to an OTel key
func translateDatadogKeyToOTel(k string) string {
	if otelKey, ok := datadogKnownResourceAttributes[strings.ToLower(k)]; ok {
		return otelKey
	}
	return k
}

type StringPool struct {
	sync.RWMutex
	pool map[string]string
}

func newStringPool() *StringPool {
	return &StringPool{
		pool: make(map[string]string),
	}
}

func (s *StringPool) Intern(str string) string {
	s.RLock()
	interned, ok := s.pool[str]
	s.RUnlock()

	if ok {
		return interned
	}

	s.Lock()
	// Double check if another goroutine has added the string after releasing the read lock
	interned, ok = s.pool[str]
	if !ok {
		interned = str
		s.pool[str] = str
	}
	s.Unlock()

	return interned
}

type attributes struct {
	resource pcommon.Map
	scope    pcommon.Map
	dp       pcommon.Map
}

func tagsToAttributes(tags []string, host string, stringPool *StringPool) attributes {
	attrs := attributes{
		resource: pcommon.NewMap(),
		scope:    pcommon.NewMap(),
		dp:       pcommon.NewMap(),
	}

	if host != "" {
		attrs.resource.PutStr(semconv.AttributeHostName, host)
	}

	var key, val string
	for _, tag := range tags {
		key, val = translateDatadogTagToKeyValuePair(tag)
		if attr, ok := datadogKnownResourceAttributes[key]; ok {
			val = stringPool.Intern(val) // No need to intern the key if we already have it
			attrs.resource.PutStr(attr, val)
		} else {
			key = stringPool.Intern(translateDatadogKeyToOTel(key))
			val = stringPool.Intern(val)
			attrs.dp.PutStr(key, val)
		}
	}

	return attrs
}
