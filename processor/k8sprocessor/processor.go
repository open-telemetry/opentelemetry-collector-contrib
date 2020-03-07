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
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/open-telemetry/opentelemetry-collector/client"
	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	sourceFormatJaeger string = "jaeger"
	sourceFormatZipkin string = "zipkin"
	ipLabelName        string = "ip"
)

type kubernetesprocessor struct {
	logger          *zap.Logger
	nextConsumer    consumer.TraceConsumer
	kc              kube.Client
	passthroughMode bool
	rules           kube.ExtractionRules
	filters         kube.Filters
}

// NewTraceProcessor returns a processor.TraceProcessor that adds the WithAttributeMap(attributes) to all spans
// passed to it.
func NewTraceProcessor(
	logger *zap.Logger,
	nextConsumer consumer.TraceConsumer,
	kubeClient kube.ClientProvider,
	options ...Option,
) (processor.TraceProcessor, error) {
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
		kc, err := kubeClient(logger, kp.rules, kp.filters, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		kp.kc = kc
	}
	return kp, nil
}

func (kp *kubernetesprocessor) GetCapabilities() processor.Capabilities {
	return processor.Capabilities{MutatesConsumedData: true}
}

func (kp *kubernetesprocessor) Start(host component.Host) error {
	go kp.kc.Start()
	return nil
}

func (kp *kubernetesprocessor) Shutdown() error {
	kp.kc.Stop()
	return nil
}

func (kp *kubernetesprocessor) ConsumeTraceData(ctx context.Context, td consumerdata.TraceData) error {
	var podIP string
	// check if the application, a collector/agent or a prior processor has already
	// annotated the batch with IP.
	if td.Resource != nil {
		podIP = td.Resource.Labels[ipLabelName]
	}

	// Jaeger client libs tag the process with the process/resource IP and
	// jaeger to OC translator maps jaeger process to OC node.
	// TODO: Should jaeger translator map jaeger process to OC resource instead?
	if podIP == "" && td.SourceFormat == sourceFormatJaeger {
		if td.Node != nil {
			podIP = td.Node.Attributes[ipLabelName]
		}
	}

	// If this was passed using Zipkin format, the information used for tagging might be present
	// in each span attribute and should be tagged not on a resource, but rather span level
	if podIP == "" && td.SourceFormat == sourceFormatZipkin {
		for _, span := range td.Spans {
			_ = kp.consumeZipkinSpan(ctx, span)
		}
	} else {
		_ = kp.consumeTraceBatch(podIP, ctx, &td)
	}

	// TODO: should add to spans that have a resource not the same as the batch?
	return kp.nextConsumer.ConsumeTraceData(ctx, td)
}

func (kp *kubernetesprocessor) consumeTraceBatch(podIP string, ctx context.Context, td *consumerdata.TraceData) error {
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
		td.Resource.Labels[ipLabelName] = podIP
	}

	// Don't invoke any k8s client functionality in passthrough mode.
	// Just tag the IP and forward the batch.
	if kp.passthroughMode {
		return nil
	}

	attrs := kp.getAttributesForPodIP(podIP)
	if len(attrs) == 0 {
		return nil
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

	return nil
}

func (kp *kubernetesprocessor) consumeZipkinSpan(ctx context.Context, span *tracepb.Span) error {
	podIP := ""
	if span.Attributes != nil && span.Attributes.AttributeMap != nil {
		value := span.Attributes.AttributeMap[ipLabelName]
		if value != nil {
			podIP = value.GetStringValue().Value
		}
	}

	if span.Attributes == nil {
		span.Attributes = &tracepb.Span_Attributes{}
	}

	if span.Attributes.AttributeMap == nil {
		span.Attributes.AttributeMap = make(map[string]*tracepb.AttributeValue, 0)
	}

	// Check if the receiver detected client IP.
	if podIP == "" {
		if c, ok := client.FromContext(ctx); ok {
			podIP = c.IP
			span.Attributes.AttributeMap[ipLabelName] = &tracepb.AttributeValue{
				Value: &tracepb.AttributeValue_StringValue{
					StringValue: &tracepb.TruncatableString{
						Value: podIP}}}
		}
	}

	// Don't invoke any k8s client functionality in passthrough mode.
	// Just tag the IP and forward the batch.
	if kp.passthroughMode {
		return nil
	}

	attrs := kp.getAttributesForPodIP(podIP)
	if len(attrs) == 0 {
		return nil
	}

	for k, v := range attrs {
		span.Attributes.AttributeMap[k] = &tracepb.AttributeValue{
			Value: &tracepb.AttributeValue_StringValue{
				StringValue: &tracepb.TruncatableString{
					Value: v}}}
	}

	return nil
}

func (kp *kubernetesprocessor) getAttributesForPodIP(ip string) map[string]string {
	pod, ok := kp.kc.GetPodByIP(ip)
	if !ok {
		return nil
	}
	return pod.Attributes
}
