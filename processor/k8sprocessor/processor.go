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

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	sourceFormatJaeger string = "jaeger"
	k8sIPLabelName     string = "k8s.pod.ip"
	clientIPLabelName  string = "ip"
)

type kubernetesprocessor struct {
	logger          *zap.Logger
	nextConsumer    consumer.TraceConsumerOld
	kc              kube.Client
	passthroughMode bool
	rules           kube.ExtractionRules
	filters         kube.Filters
}

// NewTraceProcessor returns a component.TraceProcessorOld that adds the WithAttributeMap(attributes) to all spans
// passed to it.
func NewTraceProcessor(
	logger *zap.Logger,
	nextConsumer consumer.TraceConsumerOld,
	kubeClient kube.ClientProvider,
	options ...Option,
) (component.TraceProcessorOld, error) {
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
		kc, err := kubeClient(logger, kp.rules, kp.filters, nil, nil)
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

func (kp *kubernetesprocessor) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	var podIP string
	// check if the application, a collector/agent or a prior processor has already
	// annotated the batch with IP.
	if td.Resource != nil {
		podIP = kp.k8sIPFromAttributes(td.Resource.Labels)
	}

	// Jaeger client libs tag the process with the process/resource IP and
	// jaeger to OC translator maps jaeger process to OC node.
	// TODO: Should jaeger translator map jaeger process to OC resource instead?
	if podIP == "" && td.SourceFormat == sourceFormatJaeger {
		if td.Node != nil {
			podIP = kp.k8sIPFromAttributes(td.Node.Attributes)
		}
	}

	// Check if the receiver detected client IP.
	if podIP == "" {
		if c, ok := client.FromContext(ctx); ok {
			podIP = c.IP
		}
	}

	if podIP != "" {
		if td.Resource == nil {
			td.Resource = &resourcepb.Resource{}
		}
		if td.Resource.Labels == nil {
			td.Resource.Labels = map[string]string{}
		}
		td.Resource.Labels[k8sIPLabelName] = podIP
	}

	// Don't invoke any k8s client functionality in passthrough mode.
	// Just tag the IP and forward the batch.
	if kp.passthroughMode {
		return kp.nextConsumer.ConsumeTraceData(ctx, td)
	}

	attrs := kp.getAttributesForPodIP(podIP)
	if len(attrs) == 0 {
		return kp.nextConsumer.ConsumeTraceData(ctx, td)
	}

	if td.Resource == nil {
		td.Resource = &resourcepb.Resource{}
	}
	if td.Resource.Labels == nil {
		td.Resource.Labels = map[string]string{}
	}

	for k, v := range attrs {
		td.Resource.Labels[k] = v
	}

	// TODO: should add to spans that have a resource not the same as the batch?
	return kp.nextConsumer.ConsumeTraceData(ctx, td)
}

func (kp *kubernetesprocessor) getAttributesForPodIP(ip string) map[string]string {
	pod, ok := kp.kc.GetPodByIP(ip)
	if !ok {
		return nil
	}
	return pod.Attributes
}

func (kp *kubernetesprocessor) k8sIPFromAttributes(attrs map[string]string) string {
	ip := attrs[k8sIPLabelName]
	if ip == "" {
		ip = attrs[clientIPLabelName]
	}
	return ip
}
