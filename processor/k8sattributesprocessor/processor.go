// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	logger          *zap.Logger
	apiConfig       k8sconfig.APIConfig
	kc              kube.Client
	passthroughMode bool
	rules           kube.ExtractionRules
	filters         kube.Filters
	podAssociations []kube.Association
	podIgnore       kube.Excludes
}

func (kp *kubernetesprocessor) initKubeClient(logger *zap.Logger, kubeClient kube.ClientProvider) error {
	if kubeClient == nil {
		kubeClient = kube.New
	}
	if !kp.passthroughMode {
		kc, err := kubeClient(logger, kp.apiConfig, kp.rules, kp.filters, kp.podAssociations, kp.podIgnore, nil, nil, nil, nil)
		if err != nil {
			return err
		}
		kp.kc = kc
	}
	return nil
}

func (kp *kubernetesprocessor) Start(_ context.Context, _ component.Host) error {
	if kp.rules.StartTime {
		kp.logger.Warn("k8s.pod.start_time value will be changed to use RFC3339 format in v0.83.0. " +
			"see https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/24016 for more information. " +
			"enable feature-gate k8sattr.rfc3339 to opt into this change.")
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

// processResource adds Pod metadata tags to resource based on pod association configuration
func (kp *kubernetesprocessor) processResource(ctx context.Context, resource pcommon.Resource) {
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

	if podIdentifierValue.IsNotEmpty() {
		if pod, ok := kp.kc.GetPod(podIdentifierValue); ok {
			kp.logger.Debug("getting the pod", zap.Any("pod", pod))

			for key, val := range pod.Attributes {
				if _, found := resource.Attributes().Get(key); !found {
					resource.Attributes().PutStr(key, val)
				}
			}
			kp.addContainerAttributes(resource.Attributes(), pod)
		}
	}

	namespace := stringAttributeFromMap(resource.Attributes(), conventions.AttributeK8SNamespaceName)
	if namespace != "" {
		attrsToAdd := kp.getAttributesForPodsNamespace(namespace)
		for key, val := range attrsToAdd {
			if _, found := resource.Attributes().Get(key); !found {
				resource.Attributes().PutStr(key, val)
			}
		}
	}
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
		if containerStatus, ok := containerSpec.Statuses[runID]; ok && containerStatus.ContainerID != "" {
			if _, found := attrs.Get(conventions.AttributeContainerID); !found {
				attrs.PutStr(conventions.AttributeContainerID, containerStatus.ContainerID)
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
