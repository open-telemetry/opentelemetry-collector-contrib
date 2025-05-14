// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.16.0"
)

// See:
// https://docs.datadoghq.com/opentelemetry/schema_semantics/semantic_mapping/
// https://github.com/DataDog/opentelemetry-mapping-go/blob/main/pkg/otlp/attributes/attributes.go
var datadogKnownResourceAttributes = map[string]string{
	"env":     string(semconv.DeploymentEnvironmentKey),
	"service": string(semconv.ServiceNameKey),
	"version": string(semconv.ServiceVersionKey),

	// Container-related attributes
	"container_id":   string(semconv.ContainerIDKey),
	"container_name": string(semconv.ContainerNameKey),
	"image_name":     string(semconv.ContainerImageNameKey),
	"image_tag":      string(semconv.ContainerImageTagKey),
	"runtime":        string(semconv.ContainerRuntimeKey),

	// Cloud-related attributes
	"cloud_provider": string(semconv.CloudProviderKey),
	"region":         string(semconv.CloudRegionKey),
	"zone":           string(semconv.CloudAvailabilityZoneKey),

	// ECS-related attributes
	"task_family":        string(semconv.AWSECSTaskFamilyKey),
	"task_arn":           string(semconv.AWSECSTaskARNKey),
	"ecs_cluster_name":   string(semconv.AWSECSClusterARNKey),
	"task_version":       string(semconv.AWSECSTaskRevisionKey),
	"ecs_container_name": string(semconv.AWSECSContainerARNKey),

	// K8-related attributes
	"kube_container_name": string(semconv.K8SContainerNameKey),
	"kube_cluster_name":   string(semconv.K8SClusterNameKey),
	"kube_deployment":     string(semconv.K8SDeploymentNameKey),
	"kube_replica_set":    string(semconv.K8SReplicaSetNameKey),
	"kube_stateful_set":   string(semconv.K8SStatefulSetNameKey),
	"kube_daemon_set":     string(semconv.K8SDaemonSetNameKey),
	"kube_job":            string(semconv.K8SJobNameKey),
	"kube_cronjob":        string(semconv.K8SCronJobNameKey),
	"kube_namespace":      string(semconv.K8SNamespaceNameKey),
	"pod_name":            string(semconv.K8SPodNameKey),

	// Other
	"process_id":       string(semconv.ProcessPIDKey),
	"error.stacktrace": string(semconv.ExceptionStacktraceKey),
	"error.msg":        string(semconv.ExceptionMessageKey),
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
		attrs.resource.PutStr(string(semconv.HostNameKey), host)
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
