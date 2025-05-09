// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

const (
	clientIPLabelName string = "ip"
)

type kubernetesprocessor struct {
	cfg                    component.Config
	options                []option
	telemetrySettings      component.TelemetrySettings
	logger                 *zap.Logger
	apiConfig              k8sconfig.APIConfig
	kc                     kube.Client
	passthroughMode        bool
	rules                  kube.ExtractionRules
	filters                kube.Filters
	podAssociations        []kube.Association
	podIgnore              kube.Excludes
	waitForMetadata        bool
	waitForMetadataTimeout time.Duration
}

func (kp *kubernetesprocessor) initKubeClient(set component.TelemetrySettings, kubeClient kube.ClientProvider) error {
	if kubeClient == nil {
		kubeClient = kube.New
	}
	if !kp.passthroughMode {
		kc, err := kubeClient(set, kp.apiConfig, kp.rules, kp.filters, kp.podAssociations, kp.podIgnore, nil, nil, nil, nil, kp.waitForMetadata, kp.waitForMetadataTimeout)
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
			kp.logger.Error("Could not apply option", zap.Error(err))
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			return err
		}
	}

	// This might have been set by an option already
	if kp.kc == nil {
		err := kp.initKubeClient(kp.telemetrySettings, kubeClientProvider)
		if err != nil {
			kp.logger.Error("Could not initialize kube client", zap.Error(err))
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			return err
		}
	}
	if !kp.passthroughMode {
		err := kp.kc.Start()
		if err != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			return err
		}
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

	return md, nil
}

// processLogs process logs and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		kp.processResource(ctx, rl.At(i).Resource())
	}

	return ld, nil
}

// processProfiles process profiles and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processProfiles(ctx context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) {
	rp := pd.ResourceProfiles()
	for i := 0; i < rp.Len(); i++ {
		kp.processResource(ctx, rp.At(i).Resource())
	}

	return pd, nil
}

// processResource adds Pod metadata tags to resource based on pod association configuration
func (kp *kubernetesprocessor) processResource(ctx context.Context, resource pcommon.Resource) {
	podIdentifierValue := extractPodID(ctx, resource.Attributes(), kp.podAssociations)
	kp.logger.Debug("evaluating pod identifier", zap.Any("value", podIdentifierValue))

	for i := range podIdentifierValue {
		if podIdentifierValue[i].Source.From == kube.ConnectionSource && podIdentifierValue[i].Value != "" {
			setResourceAttribute(resource.Attributes(), kube.K8sIPLabelName, podIdentifierValue[i].Value)
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
				setResourceAttribute(resource.Attributes(), key, val)
			}
			kp.addContainerAttributes(resource.Attributes(), pod)
		}
	}

	namespace := getNamespace(pod, resource.Attributes())
	if namespace != "" {
		attrsToAdd := kp.getAttributesForPodsNamespace(namespace)
		for key, val := range attrsToAdd {
			setResourceAttribute(resource.Attributes(), key, val)
		}
	}

	nodeName := getNodeName(pod, resource.Attributes())
	if nodeName != "" {
		attrsToAdd := kp.getAttributesForPodsNode(nodeName)
		for key, val := range attrsToAdd {
			setResourceAttribute(resource.Attributes(), key, val)
		}
		nodeUID := kp.getUIDForPodsNode(nodeName)
		if nodeUID != "" {
			setResourceAttribute(resource.Attributes(), conventions.AttributeK8SNodeUID, nodeUID)
		}
	}
}

func setResourceAttribute(attributes pcommon.Map, key string, val string) {
	attr, found := attributes.Get(key)
	if !found || attr.AsString() == "" {
		attributes.PutStr(key, val)
	}
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
	// if there is only one container in the pod, we can fall back to that container
	case len(pod.Containers.ByID) == 1:
		for _, c := range pod.Containers.ByID {
			containerSpec = c
		}
	case len(pod.Containers.ByName) == 1:
		for _, c := range pod.Containers.ByName {
			containerSpec = c
		}
	default:
		return
	}
	if containerSpec.Name != "" {
		setResourceAttribute(attrs, conventions.AttributeK8SContainerName, containerSpec.Name)
	}
	if containerSpec.ImageName != "" {
		setResourceAttribute(attrs, conventions.AttributeContainerImageName, containerSpec.ImageName)
	}
	if containerSpec.ImageTag != "" {
		setResourceAttribute(attrs, conventions.AttributeContainerImageTag, containerSpec.ImageTag)
	}
	if len(containerSpec.Ports) > 0 {
		for _, containerPort := range containerSpec.Ports {
			attrs.PutEmptySlice(containerPorts).AppendEmpty().SetInt(int64(containerPort))
		}
	}
	if containerSpec.CPURequest != "" {
		setResourceAttribute(attrs, containerCPURequest, containerSpec.CPURequest)
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
