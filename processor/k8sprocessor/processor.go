// Copyright 2019 Omnition Authors
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

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	k8sIPLabelName    string = "k8s.pod.ip"
	clientIPLabelName string = "ip"
)

type kubernetesprocessor struct {
	logger          *zap.Logger
	apiConfig       k8sconfig.APIConfig
	nextConsumer    consumer.TraceConsumer
	kc              kube.Client
	passthroughMode bool
	rules           kube.ExtractionRules
	filters         kube.Filters
}

var _ (component.TraceProcessor) = (*kubernetesprocessor)(nil)

// NewTraceProcessor returns a component.TraceProcessorOld that adds the WithAttributeMap(attributes) to all spans
// passed to it.
func NewTraceProcessor(
	logger *zap.Logger,
	nextConsumer consumer.TraceConsumer,
	kubeClient kube.ClientProvider,
	options ...Option,
) (component.TraceProcessor, error) {
	kp := &kubernetesprocessor{logger: logger, nextConsumer: nextConsumer}
	for _, opt := range options {
		if err := opt(kp); err != nil {
			return nil, err
		}
	}

	if kubeClient == nil {
		kubeClient = kube.New
	}
	if !kp.passthroughMode {
		kc, err := kubeClient(logger, kp.apiConfig, kp.rules, kp.filters, nil, nil)
		if err != nil {
			return nil, err
		}
		kp.kc = kc
	}
	return kp, nil
}

func (kp *kubernetesprocessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
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

func (kp *kubernetesprocessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	rss := td.ResourceSpans()
	kp.logger.Info("received rss len: ", zap.Int("size", rss.Len()))
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if rs.IsNil() {
			continue
		}

		var podIP string
		resource := rs.Resource()

		// check if the application, a collector/agent or a prior processor has already
		// annotated the batch with IP.
		if !resource.IsNil() {
			podIP = kp.k8sIPFromAttributes(resource.Attributes())
		}

		// Check if the receiver detected client IP.
		if podIP == "" {
			if c, ok := client.FromContext(ctx); ok {
				podIP = c.IP
			}
		}

		if podIP != "" {
			if resource.IsNil() {
				resource.InitEmpty()
			}
			resource.Attributes().InsertString(k8sIPLabelName, podIP)
		}

		// Don't invoke any k8s client functionality in passthrough mode.
		// Just tag the IP and forward the batch.
		if kp.passthroughMode {
			continue
		}

		// add k8s tags to resource
		attrsToAdd := kp.getAttributesForPodIP(podIP)
		if len(attrsToAdd) == 0 {
			continue
		}

		if resource.IsNil() {
			resource.InitEmpty()
		}

		attrs := resource.Attributes()
		for k, v := range attrsToAdd {
			attrs.InsertString(k, v)
		}
	}

	return kp.nextConsumer.ConsumeTraces(ctx, td)
}

func (kp *kubernetesprocessor) getAttributesForPodIP(ip string) map[string]string {
	pod, ok := kp.kc.GetPodByIP(ip)
	if !ok {
		return nil
	}
	return pod.Attributes
}

func (kp *kubernetesprocessor) k8sIPFromAttributes(attrs pdata.AttributeMap) string {
	ip := stringAttributeFromMap(attrs, k8sIPLabelName)
	if ip == "" {
		ip = stringAttributeFromMap(attrs, clientIPLabelName)
	}
	return ip
}

func stringAttributeFromMap(attrs pdata.AttributeMap, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pdata.AttributeValueSTRING {
			return val.StringVal()
		}
	}
	return ""
}
