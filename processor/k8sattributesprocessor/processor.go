// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/lru"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/moid"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/redis"
)

const (
	clientIPLabelName string = "ip"
)

type kubernetesprocessor struct {
	cfg               component.Config
	options           []option
	telemetrySettings component.TelemetrySettings
	logger            *zap.Logger
	apiConfig         k8sconfig.APIConfig
	kc                kube.Client
	passthroughMode   bool
	rules             kube.ExtractionRules
	filters           kube.Filters
	addons            []kube.AddOnMetadata
	redisConfig       redis.OpsrampRedisConfig
	podAssociations   []kube.Association
	podIgnore         kube.Excludes
	redisClient       *redis.Client
}

func (kp *kubernetesprocessor) initKubeClient(set component.TelemetrySettings, kubeClient kube.ClientProvider) error {
	if kubeClient == nil {
		kubeClient = kube.New
	}
	if !kp.passthroughMode {
		kc, err := kubeClient(set, kp.apiConfig, kp.rules, kp.filters, kp.podAssociations, kp.podIgnore, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		kp.kc = kc
	}
	return nil
}

func (kp *kubernetesprocessor) Start(_ context.Context, host component.Host) error {
	allOptions := append(createProcessorOpts(kp.cfg), kp.options...)

	for _, opt := range allOptions {
		if err := opt(kp); err != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			return nil
		}
	}

	kp.logger.Info("ops k8s attr processor start", zap.Any("redisHost", kp.redisConfig.RedisHost),
		zap.Any("redisPort", kp.redisConfig.RedisPort),
		zap.Any("redisPass", kp.redisConfig.RedisPass))

	cache := lru.GetInstance(kp.redisConfig.LruCacheSize, kp.redisConfig.LruExpirationTime)

	if cache == nil {
		kp.logger.Error("Failed to initilize the cache with GetInstance()")
		return nil
	}

	kp.redisClient = redis.NewClient(kp.logger, cache, kp.redisConfig.RedisHost, kp.redisConfig.RedisPort, kp.redisConfig.RedisPass)

	// This might have been set by an option already
	if kp.kc == nil {
		err := kp.initKubeClient(kp.telemetrySettings, kubeClientProvider)
		if err != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			return nil
		}
	}
	if !kp.passthroughMode {
		go kp.kc.Start()
	}
	return nil
}

func (kp *kubernetesprocessor) Shutdown(context.Context) error {
	if kp.kc == nil {
		return nil
	}
	if !kp.passthroughMode {
		kp.kc.Stop()
	}
	return nil
}

// processTraces process traces and add k8s metadata using resource IP or incoming IP as pod origin.
func (kp *kubernetesprocessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		kp.processResource(ctx, rss.At(i).Resource())
	}

	return td, nil
}

// processMetrics process metrics and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		kp.processResource(ctx, rm.At(i).Resource())
	}

	kp.filterOnlyOpsrampMetrics(md)

	return md, nil
}

// processLogs process logs and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		kp.processResource(ctx, rl.At(i).Resource())

		kp.processEventBody(rl.At(i))
	}

	return ld, nil
}

// processResource adds Pod metadata tags to resource based on pod association configuration
func (kp *kubernetesprocessor) processResource(ctx context.Context, resource pcommon.Resource) {
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		kp.logger.Debug("res-attributes", zap.Any(k, v.Str()))
		return true
	})

	if val, found := resource.Attributes().Get("type"); found && val.Str() == "event" {
		if kind, found := resource.Attributes().Get("k8s.object.kind"); found {
			if objectuid, found := resource.Attributes().Get("k8s.object.uid"); found {
				if kind.Str() == "Pod" {
					resource.Attributes().PutStr("k8s.pod.uid", objectuid.Str())
				} else if kind.Str() == "Node" {
					resource.Attributes().PutStr("k8s.node.uid", objectuid.Str())
				}
			}
		}
	}

	podIdentifierValue := extractPodID(ctx, resource.Attributes(), kp.podAssociations)
	kp.logger.Debug("evaluating pod identifier", zap.Any("value", podIdentifierValue))

	for i := range podIdentifierValue {
		if podIdentifierValue[i].Source.From == kube.ConnectionSource && podIdentifierValue[i].Value != "" {
			if _, found := resource.Attributes().Get(kube.K8sIPLabelName); !found {
				resource.Attributes().PutStr(kube.K8sIPLabelName, podIdentifierValue[i].Value)
			}
			break
		}
	}
	if kp.passthroughMode {
		return
	}

	var pod *kube.Pod
	if podIdentifierValue.IsNotEmpty() {
		var podFound bool
		if pod, podFound = kp.kc.GetPod(podIdentifierValue); podFound {
			kp.logger.Debug("getting the pod", zap.Any("pod", pod))

			for key, val := range pod.Attributes {
				if _, found := resource.Attributes().Get(key); !found {
					resource.Attributes().PutStr(key, val)
				}
			}
			kp.addContainerAttributes(resource.Attributes(), pod)
		}
	}

	namespace := getNamespace(pod, resource.Attributes())
	if namespace != "" {
		attrsToAdd := kp.getAttributesForPodsNamespace(namespace)
		for key, val := range attrsToAdd {
			if _, found := resource.Attributes().Get(key); !found {
				resource.Attributes().PutStr(key, val)
			}
		}
	}

	nodeName := getNodeName(pod, resource.Attributes())
	if nodeName != "" {
		attrsToAdd := kp.getAttributesForPodsNode(nodeName)
		for key, val := range attrsToAdd {
			if _, found := resource.Attributes().Get(key); !found {
				resource.Attributes().PutStr(key, val)
			}
		}
		nodeUID := kp.getUIDForPodsNode(nodeName)
		if nodeUID != "" {
			if _, found := resource.Attributes().Get(conventions.AttributeK8SNodeUID); !found {
				resource.Attributes().PutStr(conventions.AttributeK8SNodeUID, nodeUID)
			}
		}
	}
	kp.processopsrampResources(ctx, resource)
	kp.addOpsrampEventResourceAttributes(ctx, resource)
}

func (kp *kubernetesprocessor) processEventBody(resourceLogs plog.ResourceLogs) {
	if val, found := resourceLogs.Resource().Attributes().Get("type"); found && val.Str() == "event" {

		bodyMap := map[string]string{}

		resourceLogs.Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
			bodyMap[k] = v.Str()
			return true
		})

		ilss := resourceLogs.ScopeLogs()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			logs := ils.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)

				lr.Attributes().Range(func(k string, v pcommon.Value) bool {
					bodyMap[k] = v.Str()
					return true
				})

				bodyMap["message"] = lr.Body().AsString()

				body, err := json.Marshal(bodyMap)
				if err != nil {
					kp.logger.Error("Failed to marshal attributes as body ")
				}

				lr.Body().SetStr(string(body))
			}
		}
	}
}

func (kp *kubernetesprocessor) addOpsrampEventResourceAttributes(ctx context.Context, resource pcommon.Resource) {

	if val, found := resource.Attributes().Get("type"); found && val.Str() == "event" {
		resource.Attributes().PutStr("source", "kubernetes")
		resource.Attributes().PutStr("level", "Unknown")
		resource.Attributes().PutStr("cluster_name", kp.redisConfig.ClusterName)
		resource.Attributes().PutStr("host", kp.redisConfig.ClusterName)
		resource.Attributes().PutStr("resourceUUID", kp.redisConfig.ClusterUid)

		if val, found := resource.Attributes().Get("k8s.namespace.name"); found {
			resource.Attributes().PutStr("namespace", val.Str())
		}

		host := ""
		if val, found := resource.Attributes().Get(semconv.AttributeK8SNodeName); found {
			host = val.Str()
			if host != "" {
				//overwrite node opsramp resource UUID in resourceUUID
				resource.Attributes().PutStr("host", host)

				if resourceUuid := kp.GetResourceUuidUsingResourceNodeMoid(ctx, resource); resourceUuid != "" {
					resource.Attributes().PutStr("resourceUUID", resourceUuid)
				}
			}
		}
	}
}

// processResource adds Pod metadata tags to resource based on pod association configuration
func (kp *kubernetesprocessor) processopsrampResources(ctx context.Context, resource pcommon.Resource) {
	var found bool
	var resourceUuid string

	for _, addon := range kp.addons {
		// If receiver has already added some attributes with some value, then we do not overwrite here.
		// For ex. type = event is already added for kube events. We should not overwrite it with type = RESOURCE.
		kp.logger.Debug("addon", zap.Any("key", addon.Key))

		if _, found := resource.Attributes().Get(addon.Key); !found {
			kp.logger.Debug("addon not found adding it", zap.Any("key", addon.Key))

			resource.Attributes().PutStr(addon.Key, addon.Value)
		}
	}

	if _, found = resource.Attributes().Get("k8s.pod.uid"); found {
		if resourceUuid = kp.GetResourceUuidUsingPodMoid(ctx, resource); resourceUuid == "" {
			if podname, found := resource.Attributes().Get("k8s.pod.name"); found {
				kp.logger.Debug("opsramp resourceuuid not found in redis", zap.Any("podname", podname.Str()))
			}
		}
	} else if nodename, found := resource.Attributes().Get("k8s.node.name"); found {
		if resourceUuid = kp.GetResourceUuidUsingResourceNodeMoid(ctx, resource); resourceUuid == "" {
			kp.logger.Debug("opsramp resourceuuid not found in redis", zap.Any("nodename", nodename.Str()))
		}
	} else if dpname, found := resource.Attributes().Get("k8s.deployment.name"); found {
		if resourceUuid = kp.GetResourceUuidUsingWorkloadMoid(ctx, resource); resourceUuid == "" {
			kp.logger.Debug("opsramp resourceuuid not found in redis", zap.Any("deployment", dpname.Str()))
		}
	} else if dsname, found := resource.Attributes().Get("k8s.daemonset.name"); found {
		if resourceUuid = kp.GetResourceUuidUsingWorkloadMoid(ctx, resource); resourceUuid == "" {
			kp.logger.Debug("opsramp resourceuuid not found in redis", zap.Any("daemonset", dsname.Str()))
		}
	} else if rsname, found := resource.Attributes().Get("k8s.replicaset.name"); found {
		if resourceUuid = kp.GetResourceUuidUsingWorkloadMoid(ctx, resource); resourceUuid == "" {
			kp.logger.Debug("opsramp resourceuuid not found in redis", zap.Any("replicaset", rsname.Str()))
		}
	} else if ssname, found := resource.Attributes().Get("k8s.statefulset.name"); found {
		if resourceUuid = kp.GetResourceUuidUsingWorkloadMoid(ctx, resource); resourceUuid == "" {
			kp.logger.Debug("opsramp resourceuuid not found in redis", zap.Any("statefulset", ssname.Str()))
		}
	} else {
		if resourceUuid = kp.redisConfig.ClusterUid; resourceUuid == "" {
			kp.logger.Debug("opsramp resourceuuid not found", zap.Any("clustername", kp.redisConfig.ClusterName))
		}

		/*
			No need to get it from redis. As its directly available in config.
			if resourceUuid = kp.GetResourceUuidUsingClusterMoid(ctx, resource); resourceUuid == "" {
				kp.logger.Debug("opsramp resourceuuid not found in redis", zap.Any("clustername", kp.redisConfig.ClusterName))
			}
		*/
	}

	if resourceUuid != "" {
		resource.Attributes().PutStr("uuid", resourceUuid)
	}

}

func (kp *kubernetesprocessor) filterOnlyOpsrampMetrics(md pmetric.Metrics) {
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		resource := rmetrics.Resource()
		if _, found := resource.Attributes().Get("uuid"); !found {
			return true
		}
		return false
	})
}

func (op *kubernetesprocessor) GetResourceUuidUsingPodMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {
	var namespace, podname pcommon.Value
	var found bool

	if namespace, found = resource.Attributes().Get("k8s.namespace.name"); !found {
		return
	}
	if podname, found = resource.Attributes().Get("k8s.pod.name"); !found {
		return
	}

	podMoid := moid.NewMoid(op.redisConfig.ClusterName).WithNamespaceName(namespace.Str()).WithPodName(podname.Str())

	//removed workload name in POD MoId
	/*if rsname, found = resource.Attributes().Get("k8s.replicaset.name"); found {
		podMoid.WithReplicasetName(rsname.Str())
	} else if dsname, found = resource.Attributes().Get("k8s.daemonset.name"); found {
		podMoid.WithDaemonsetName(dsname.Str())
	} else if ssname, found = resource.Attributes().Get("k8s.statefulset.name"); found {
		podMoid.WithStatefulsetName(ssname.Str())
	}*/

	podMoidKey := podMoid.PodMoid()

	resourceUuid = op.redisClient.GetUuidValueInString(ctx, podMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", podMoidKey), zap.Any("value", resourceUuid))
	return
}

func (op *kubernetesprocessor) GetResourceUuidUsingWorkloadMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {
	var namespace, rsname, dsname, ssname, dpname pcommon.Value
	var found bool

	if namespace, found = resource.Attributes().Get("k8s.namespace.name"); !found {
		return
	}

	workloadMoid := moid.NewMoid(op.redisConfig.ClusterName).WithNamespaceName(namespace.Str())

	var workloadMoidKey string
	if rsname, found = resource.Attributes().Get("k8s.replicaset.name"); found {
		workloadMoidKey = workloadMoid.WithReplicasetName(rsname.Str()).ReplicaSetMoid()
	} else if dsname, found = resource.Attributes().Get("k8s.daemonset.name"); found {
		workloadMoidKey = workloadMoid.WithDaemonsetName(dsname.Str()).DaemonSetMoid()
	} else if ssname, found = resource.Attributes().Get("k8s.statefulset.name"); found {
		workloadMoidKey = workloadMoid.WithStatefulsetName(ssname.Str()).StatefulSetMoid()
	} else if dpname, found = resource.Attributes().Get("k8s.deployment.name"); found {
		workloadMoidKey = workloadMoid.WithDeploymentName(dpname.Str()).DeploymentMoid()
	}
	if workloadMoidKey == "" {
		return
	}

	resourceUuid = op.redisClient.GetUuidValueInString(ctx, workloadMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", workloadMoidKey), zap.Any("value", resourceUuid))
	return
}

func (op *kubernetesprocessor) GetResourceUuidUsingResourceNodeMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {
	var nodename pcommon.Value
	var found bool
	if nodename, found = resource.Attributes().Get("k8s.node.name"); !found {
		return
	}

	nodeMoidKey := moid.NewMoid(op.redisConfig.ClusterName).WithNodeName(nodename.Str()).NodeMoid()

	resourceUuid = op.redisClient.GetUuidValueInString(ctx, nodeMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", nodeMoidKey), zap.Any("value", resourceUuid))
	return
}

func (op *kubernetesprocessor) GetResourceUuidUsingCurrentNodeMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {

	nodeMoidKey := moid.NewMoid(op.redisConfig.ClusterName).WithNodeName(op.redisConfig.NodeName).NodeMoid()

	resourceUuid = op.redisClient.GetUuidValueInString(ctx, nodeMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", nodeMoidKey), zap.Any("value", resourceUuid))
	return
}

func (op *kubernetesprocessor) GetResourceUuidUsingClusterMoid(ctx context.Context, resource pcommon.Resource) (resourceUuid string) {

	nodeMoidKey := moid.NewMoid(op.redisConfig.ClusterName).WithClusterUuid(op.redisConfig.ClusterUid).ClusterMoid()

	resourceUuid = op.redisClient.GetUuidValueInString(ctx, nodeMoidKey)
	op.logger.Debug("redis KV ", zap.Any("key", nodeMoidKey), zap.Any("value", resourceUuid))
	return
}

func getNamespace(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.Namespace != "" {
		return pod.Namespace
	}
	return stringAttributeFromMap(resAttrs, conventions.AttributeK8SNamespaceName)
}

func getNodeName(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.NodeName != "" {
		return pod.NodeName
	}
	return stringAttributeFromMap(resAttrs, conventions.AttributeK8SNodeName)
}

// addContainerAttributes looks if pod has any container identifiers and adds additional container attributes
func (kp *kubernetesprocessor) addContainerAttributes(attrs pcommon.Map, pod *kube.Pod) {
	containerName := stringAttributeFromMap(attrs, conventions.AttributeK8SContainerName)
	containerID := stringAttributeFromMap(attrs, conventions.AttributeContainerID)
	var (
		containerSpec *kube.Container
		ok            bool
	)
	switch {
	case containerName != "":
		containerSpec, ok = pod.Containers.ByName[containerName]
		if !ok {
			return
		}
	case containerID != "":
		containerSpec, ok = pod.Containers.ByID[containerID]
		if !ok {
			return
		}
	default:
		return
	}
	if containerSpec.Name != "" {
		if _, found := attrs.Get(conventions.AttributeK8SContainerName); !found {
			attrs.PutStr(conventions.AttributeK8SContainerName, containerSpec.Name)
		}
	}
	if containerSpec.ImageName != "" {
		if _, found := attrs.Get(conventions.AttributeContainerImageName); !found {
			attrs.PutStr(conventions.AttributeContainerImageName, containerSpec.ImageName)
		}
	}
	if containerSpec.ImageTag != "" {
		if _, found := attrs.Get(conventions.AttributeContainerImageTag); !found {
			attrs.PutStr(conventions.AttributeContainerImageTag, containerSpec.ImageTag)
		}
	}
	// attempt to get container ID from restart count
	runID := -1
	runIDAttr, ok := attrs.Get(conventions.AttributeK8SContainerRestartCount)
	if ok {
		containerRunID, err := intFromAttribute(runIDAttr)
		if err != nil {
			kp.logger.Debug(err.Error())
		} else {
			runID = containerRunID
		}
	} else {
		// take the highest runID (restart count) which represents the currently running container in most cases
		for containerRunID := range containerSpec.Statuses {
			if containerRunID > runID {
				runID = containerRunID
			}
		}
	}
	if runID != -1 {
		if containerStatus, ok := containerSpec.Statuses[runID]; ok {
			if _, found := attrs.Get(conventions.AttributeContainerID); !found && containerStatus.ContainerID != "" {
				attrs.PutStr(conventions.AttributeContainerID, containerStatus.ContainerID)
			}
			if _, found := attrs.Get(containerImageRepoDigests); !found && containerStatus.ImageRepoDigest != "" {
				attrs.PutEmptySlice(containerImageRepoDigests).AppendEmpty().SetStr(containerStatus.ImageRepoDigest)
			}
		}
	}
}

func (kp *kubernetesprocessor) getAttributesForPodsNamespace(namespace string) map[string]string {
	ns, ok := kp.kc.GetNamespace(namespace)
	if !ok {
		return nil
	}
	return ns.Attributes
}

func (kp *kubernetesprocessor) getAttributesForPodsNode(nodeName string) map[string]string {
	node, ok := kp.kc.GetNode(nodeName)
	if !ok {
		return nil
	}
	return node.Attributes
}

func (kp *kubernetesprocessor) getUIDForPodsNode(nodeName string) string {
	node, ok := kp.kc.GetNode(nodeName)
	if !ok {
		return ""
	}
	return node.NodeUID
}

// intFromAttribute extracts int value from an attribute stored as string or int
func intFromAttribute(val pcommon.Value) (int, error) {
	switch val.Type() {
	case pcommon.ValueTypeInt:
		return int(val.Int()), nil
	case pcommon.ValueTypeStr:
		i, err := strconv.Atoi(val.Str())
		if err != nil {
			return 0, err
		}
		return i, nil
	case pcommon.ValueTypeEmpty, pcommon.ValueTypeDouble, pcommon.ValueTypeBool, pcommon.ValueTypeMap, pcommon.ValueTypeSlice, pcommon.ValueTypeBytes:
		fallthrough
	default:
		return 0, fmt.Errorf("wrong attribute type %v, expected int", val.Type())
	}
}
