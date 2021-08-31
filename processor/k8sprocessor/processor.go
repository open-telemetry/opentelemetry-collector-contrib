// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	k8sIPLabelName    string = "k8s.pod.ip"
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
		kc, err := kubeClient(logger, kp.apiConfig, kp.rules, kp.filters, kp.podAssociations, kp.podIgnore, nil, nil, nil)
		if err != nil {
			return err
		}
		kp.kc = kc
	}
	return nil
}

func (kp *kubernetesprocessor) Start(_ context.Context, _ component.Host) error {
	if !kp.passthroughMode {
		go kp.kc.Start()
	}
	return nil
}

func (kp *kubernetesprocessor) Shutdown(context.Context) error {
	if !kp.passthroughMode {
		kp.kc.Stop()
	}
	return nil
}

// processTraces process traces and add k8s metadata using resource IP or incoming IP as pod origin.
func (kp *kubernetesprocessor) processTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		kp.processResource(ctx, rss.At(i).Resource())
	}

	return td, nil
}

// processMetrics process metrics and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processMetrics(ctx context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		kp.processResource(ctx, rm.At(i).Resource())
	}

	return md, nil
}

// processLogs process logs and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) processLogs(ctx context.Context, ld pdata.Logs) (pdata.Logs, error) {
	rl := ld.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		kp.processResource(ctx, rl.At(i).Resource())
	}

	return ld, nil
}

// processResource adds Pod metadata tags to resource based on pod association configuration
func (kp *kubernetesprocessor) processResource(ctx context.Context, resource pdata.Resource) {
	podIdentifierKey, podIdentifierValue := extractPodID(ctx, resource.Attributes(), kp.podAssociations)
	if podIdentifierKey != "" {
		resource.Attributes().InsertString(podIdentifierKey, string(podIdentifierValue))
	}

	namespace := stringAttributeFromMap(resource.Attributes(), conventions.AttributeK8SNamespaceName)
	if namespace != "" {
		resource.Attributes().InsertString(conventions.AttributeK8SNamespaceName, namespace)
	}

	if kp.passthroughMode {
		return
	}

	if podIdentifierKey != "" {
		attrsToAdd := kp.getAttributesForPod(podIdentifierValue)
		for key, val := range attrsToAdd {
			resource.Attributes().InsertString(key, val)
		}
	}

	if namespace != "" {
		attrsToAdd := kp.getAttributesForPodsNamespace(namespace)
		for key, val := range attrsToAdd {
			resource.Attributes().InsertString(key, val)
		}
	}
}

func (kp *kubernetesprocessor) getAttributesForPod(identifier kube.PodIdentifier) map[string]string {
	pod, ok := kp.kc.GetPod(identifier)
	if !ok {
		return nil
	}
	return pod.Attributes
}

func (kp *kubernetesprocessor) getAttributesForPodsNamespace(namespace string) map[string]string {
	ns, ok := kp.kc.GetNamespace(namespace)
	if !ok {
		return nil
	}
	return ns.Attributes
}
