// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/metadata"
)

const (
	clientIPLabelName string = "ip"
)

type kubernetesprocessor struct {
	cfg                    component.Config
	options                []option
	telemetrySettings      component.TelemetrySettings
	telemetry              *metadata.TelemetryBuilder
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
		kc, err := kubeClient(set, kp.apiConfig, kp.rules, kp.filters, kp.podAssociations, kp.podIgnore, nil, kube.InformersFactoryList{}, kp.waitForMetadata, kp.waitForMetadataTimeout)
		if err != nil {
			return err
		}
		kp.kc = kc
	}
	return nil
}

func (kp *kubernetesprocessor) Start(_ context.Context, host component.Host) error {
	if metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.IsEnabled() && !metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.IsEnabled() {
		err := errors.New("processor.k8sattributes.DontEmitV0K8sConventions cannot be enabled without enabling processor.k8sattributes.EmitV1K8sConventions")
		kp.logger.Error("Invalid feature gate combination", zap.Error(err))
		componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		return err
	}

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
	if kp.telemetry != nil {
		kp.telemetry.Shutdown()
	}
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
		kp.processResource(ctx, rss.At(i).Resource(), "traces")
	}

	return td, nil
}

// processMetrics process metrics and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		kp.processResource(ctx, rm.At(i).Resource(), "metrics")
	}

	return md, nil
}

// processLogs process logs and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		kp.processResource(ctx, rl.At(i).Resource(), "logs")
	}

	return ld, nil
}

// processProfiles process profiles and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processProfiles(ctx context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) {
	rp := pd.ResourceProfiles()
	for i := 0; i < rp.Len(); i++ {
		kp.processResource(ctx, rp.At(i).Resource(), "profiles")
	}

	return pd, nil
}

// processResource adds Pod metadata tags to resource based on pod association configuration
func (kp *kubernetesprocessor) processResource(ctx context.Context, resource pcommon.Resource, signalType string) {
	podIdentifierValue := extractPodID(ctx, resource.Attributes(), kp.podAssociations)
	kp.logger.Debug("evaluating pod identifier", zap.Any("value", podIdentifierValue))

	for i := range podIdentifierValue {
		if podIdentifierValue[i].Source.From == kube.ConnectionSource && podIdentifierValue[i].Value != "" {
			if kp.passthroughMode || kp.rules.PodIP {
				setResourceAttribute(resource.Attributes(), string(conventions.K8SPodIPKey), podIdentifierValue[i].Value)
			}
			break
		}
	}
	if kp.passthroughMode {
		return
	}

	var pod *kube.Pod
	var podFound bool
	podIdentifierStr := buildPodIdentifierString(podIdentifierValue)
	if podIdentifierValue.IsNotEmpty() {
		if pod, podFound = kp.kc.GetPod(podIdentifierValue); podFound {
			kp.logger.Debug("getting the pod", zap.Any("pod", pod))

			// Record successful pod association
			if kp.telemetry != nil {
				successAttr := metric.WithAttributes(
					attribute.String("status", "success"),
					attribute.String("pod_identifier", podIdentifierStr),
					attribute.String("otelcol.signal", signalType),
				)
				kp.telemetry.K8sPodAssociation.Add(ctx, 1, successAttr)
			}

			for key, val := range pod.Attributes {
				setResourceAttribute(resource.Attributes(), key, val)
			}
			kp.addContainerAttributes(resource.Attributes(), pod)
		} else {
			// Record failed pod association
			kp.logger.Debug("pod not found", zap.Any("podIdentifier", podIdentifierValue))
			if kp.telemetry != nil {
				errorAttr := metric.WithAttributes(
					attribute.String("status", "error"),
					attribute.String("pod_identifier", podIdentifierStr),
					attribute.String("otelcol.signal", signalType),
				)
				kp.telemetry.K8sPodAssociation.Add(ctx, 1, errorAttr)
			}
		}
	} else {
		// Record failed pod association when no identifier found
		kp.logger.Debug("no pod identifier found")
		if kp.telemetry != nil {
			errorAttr := metric.WithAttributes(
				attribute.String("status", "error"),
				attribute.String("pod_identifier", podIdentifierStr),
				attribute.String("otelcol.signal", signalType),
			)
			kp.telemetry.K8sPodAssociation.Add(ctx, 1, errorAttr)
		}
	}

	namespace := getNamespace(pod, resource.Attributes())
	if namespace != "" {
		attrsToAdd := kp.getAttributesForPodsNamespace(namespace)
		for key, val := range attrsToAdd {
			setResourceAttribute(resource.Attributes(), key, val)
		}

		if kp.rules.ServiceNamespace {
			setResourceAttribute(resource.Attributes(), string(conventions.ServiceNamespaceKey), namespace)
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
			setResourceAttribute(resource.Attributes(), string(conventions.K8SNodeUIDKey), nodeUID)
		}
	}

	deployment := getDeploymentUID(pod, resource.Attributes())
	if deployment != "" {
		attrsToAdd := kp.getAttributesForPodsDeployment(deployment)
		for key, val := range attrsToAdd {
			setResourceAttribute(resource.Attributes(), key, val)
		}
	}

	statefulset := getStatefulSetUID(pod, resource.Attributes())
	if statefulset != "" {
		attrsToAdd := kp.getAttributesForPodsStatefulSet(statefulset)
		for key, val := range attrsToAdd {
			setResourceAttribute(resource.Attributes(), key, val)
		}
	}

	daemonset := getDaemonSetUID(pod, resource.Attributes())
	if daemonset != "" {
		attrsToAdd := kp.getAttributesForPodsDaemonSet(daemonset)
		for key, val := range attrsToAdd {
			setResourceAttribute(resource.Attributes(), key, val)
		}
	}

	job := getJobUID(pod, resource.Attributes())
	if job != "" {
		attrsToAdd := kp.getAttributesForPodsJob(job)
		for key, val := range attrsToAdd {
			setResourceAttribute(resource.Attributes(), key, val)
		}
	}
}

func setResourceAttribute(attributes pcommon.Map, key, val string) {
	attr, found := attributes.Get(key)
	if !found || attr.AsString() == "" {
		attributes.PutStr(key, val)
	}
}

func getNamespace(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.Namespace != "" {
		return pod.Namespace
	}
	return stringAttributeFromMap(resAttrs, string(conventions.K8SNamespaceNameKey))
}

func getNodeName(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.NodeName != "" {
		return pod.NodeName
	}
	return stringAttributeFromMap(resAttrs, string(conventions.K8SNodeNameKey))
}

func getDeploymentUID(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.DeploymentUID != "" {
		return pod.DeploymentUID
	}
	return stringAttributeFromMap(resAttrs, string(conventions.K8SDeploymentUIDKey))
}

func getStatefulSetUID(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.StatefulSetUID != "" {
		return pod.StatefulSetUID
	}
	return stringAttributeFromMap(resAttrs, string(conventions.K8SStatefulSetUIDKey))
}

func getDaemonSetUID(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.DaemonSetUID != "" {
		return pod.DaemonSetUID
	}
	return stringAttributeFromMap(resAttrs, string(conventions.K8SDaemonSetUIDKey))
}

func getJobUID(pod *kube.Pod, resAttrs pcommon.Map) string {
	if pod != nil && pod.JobUID != "" {
		return pod.JobUID
	}
	return stringAttributeFromMap(resAttrs, string(conventions.K8SJobUIDKey))
}

// addContainerAttributes looks if pod has any container identifiers and adds additional container attributes
func (kp *kubernetesprocessor) addContainerAttributes(attrs pcommon.Map, pod *kube.Pod) {
	containerName := stringAttributeFromMap(attrs, string(conventions.K8SContainerNameKey))
	containerID := stringAttributeFromMap(attrs, string(conventions.ContainerIDKey))
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
		setResourceAttribute(attrs, string(conventions.K8SContainerNameKey), containerSpec.Name)
	}
	if containerSpec.ImageName != "" {
		setResourceAttribute(attrs, string(conventions.ContainerImageNameKey), containerSpec.ImageName)
	}
	enableStable := metadata.ProcessorK8sattributesEmitV1K8sConventionsFeatureGate.IsEnabled()
	disableLegacy := metadata.ProcessorK8sattributesDontEmitV0K8sConventionsFeatureGate.IsEnabled()
	if !disableLegacy && containerSpec.ImageTag != "" {
		setResourceAttribute(attrs, containerImageTag, containerSpec.ImageTag)
	}
	if enableStable && len(containerSpec.ImageTags) > 0 {
		sliceVal := attrs.PutEmptySlice(string(conventions.ContainerImageTagsKey))
		for _, tag := range containerSpec.ImageTags {
			sliceVal.AppendEmpty().SetStr(tag)
		}
	}
	if containerSpec.ServiceInstanceID != "" {
		setResourceAttribute(attrs, string(conventions.ServiceInstanceIDKey), containerSpec.ServiceInstanceID)
	}
	if containerSpec.ServiceVersion != "" {
		setResourceAttribute(attrs, string(conventions.ServiceVersionKey), containerSpec.ServiceVersion)
	}
	// attempt to get container ID from restart count
	runID := -1
	runIDAttr, ok := attrs.Get(string(conventions.K8SContainerRestartCountKey))
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
			if _, found := attrs.Get(string(conventions.ContainerIDKey)); !found && containerStatus.ContainerID != "" {
				attrs.PutStr(string(conventions.ContainerIDKey), containerStatus.ContainerID)
			}
			if _, found := attrs.Get(string(conventions.ContainerImageRepoDigestsKey)); !found && containerStatus.ImageRepoDigest != "" {
				attrs.PutEmptySlice(string(conventions.ContainerImageRepoDigestsKey)).AppendEmpty().SetStr(containerStatus.ImageRepoDigest)
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

func (kp *kubernetesprocessor) getAttributesForPodsDeployment(deploymentUID string) map[string]string {
	d, ok := kp.kc.GetDeployment(deploymentUID)
	if !ok {
		return nil
	}
	return d.Attributes
}

func (kp *kubernetesprocessor) getAttributesForPodsStatefulSet(statefulsetUID string) map[string]string {
	d, ok := kp.kc.GetStatefulSet(statefulsetUID)
	if !ok {
		return nil
	}
	return d.Attributes
}

func (kp *kubernetesprocessor) getAttributesForPodsDaemonSet(daemonsetUID string) map[string]string {
	d, ok := kp.kc.GetDaemonSet(daemonsetUID)
	if !ok {
		return nil
	}
	return d.Attributes
}

func (kp *kubernetesprocessor) getAttributesForPodsJob(jobUID string) map[string]string {
	j, ok := kp.kc.GetJob(jobUID)
	if !ok {
		return nil
	}
	return j.Attributes
}

func (kp *kubernetesprocessor) getUIDForPodsNode(nodeName string) string {
	node, ok := kp.kc.GetNode(nodeName)
	if !ok {
		return ""
	}
	return node.NodeUID
}

// buildPodIdentifierString combines all identifier values into a comma-separated string
func buildPodIdentifierString(podIdentifierValue kube.PodIdentifier) string {
	var identifiers []string
	for i := range podIdentifierValue {
		if podIdentifierValue[i].Value != "" {
			identifiers = append(identifiers, podIdentifierValue[i].Value)
		}
	}
	if len(identifiers) > 0 {
		return strings.Join(identifiers, ",")
	}
	return "unknown"
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
	default:
		return 0, fmt.Errorf("wrong attribute type %v, expected int", val.Type())
	}
}
