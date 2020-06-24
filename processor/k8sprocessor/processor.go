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
	"net"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	k8sIPLabelName    string = "k8s.pod.ip"
	clientIPLabelName string = "ip"
)

type kubernetesprocessor struct {
	logger              *zap.Logger
	apiConfig           k8sconfig.APIConfig
	kc                  kube.Client
	passthroughMode     bool
	rules               kube.ExtractionRules
	filters             kube.Filters
	nextTraceConsumer   consumer.TraceConsumer
	nextMetricsConsumer consumer.MetricsConsumer
}

var _ (component.TraceProcessor) = (*kubernetesprocessor)(nil)
var _ (component.MetricsProcessor) = (*kubernetesprocessor)(nil)

// NewTraceProcessor returns a component.TraceProcessor that adds the WithAttributeMap(attributes) to all spans
// passed to it.
func NewTraceProcessor(
	logger *zap.Logger,
	nextTraceConsumer consumer.TraceConsumer,
	kubeClient kube.ClientProvider,
	options ...Option,
) (component.TraceProcessor, error) {
	kp := &kubernetesprocessor{logger: logger, nextTraceConsumer: nextTraceConsumer}
	for _, opt := range options {
		if err := opt(kp); err != nil {
			return nil, err
		}
	}
	err := kp.initKubeClient(logger, kubeClient)
	if err != nil {
		return nil, err
	}
	return kp, nil
}

// NewMetricsProcessor returns a component.MetricProcessor that adds the k8s attributes to metrics passed to it.
func NewMetricsProcessor(
	logger *zap.Logger,
	nextMetricsConsumer consumer.MetricsConsumer,
	kubeClient kube.ClientProvider,
	options ...Option,
) (component.MetricsProcessor, error) {
	kp := &kubernetesprocessor{logger: logger, nextMetricsConsumer: nextMetricsConsumer}
	for _, opt := range options {
		if err := opt(kp); err != nil {
			return nil, err
		}
	}
	err := kp.initKubeClient(logger, kubeClient)
	if err != nil {
		return nil, err
	}
	return kp, nil
}

func (kp *kubernetesprocessor) initKubeClient(logger *zap.Logger, kubeClient kube.ClientProvider) error {
	if kubeClient == nil {
		kubeClient = kube.New
	}
	if !kp.passthroughMode {
		kc, err := kubeClient(logger, kp.apiConfig, kp.rules, kp.filters, nil, nil)
		if err != nil {
			return err
		}
		kp.kc = kc
	}
	return nil
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

	return kp.nextTraceConsumer.ConsumeTraces(ctx, td)
}

// ConsumeMetrics process metrics and add k8s metadata using resource hostname as pod origin.
// TODO: Move to internal data model once it's available in contrib.
func (kp *kubernetesprocessor) ConsumeMetrics(ctx context.Context, metrics pdata.Metrics) error {
	mds := pdatautil.MetricsToMetricsData(metrics)

	for i := range mds {
		md := &mds[i]
		var presetPodIP string
		var podIP string

		// Check if a collector/agent or a prior processor has already annotated the metrics with IP.
		if md.Resource.GetLabels() != nil {
			presetPodIP = md.Resource.Labels[k8sIPLabelName]
		}

		// Most of the metric receivers uses "host.hostname" resource label (which is represented as
		// Node.Identifier.HostName in OpenCensus format) to identify metrics origin.
		// In k8s environment, it's set to a pod IP address. If the value doesn't represent
		// an IP address, we skip it.
		if podIP == "" && md.Node.GetIdentifier().GetHostName() != "" {
			hostname := md.Node.Identifier.HostName
			if net.ParseIP(hostname) != nil {
				podIP = hostname
			}
		}

		if presetPodIP == "" && podIP != "" {
			if md.Resource == nil {
				md.Resource = &resourcepb.Resource{}
			}
			if md.Resource.Labels == nil {
				md.Resource.Labels = map[string]string{}
			}
			md.Resource.Labels[k8sIPLabelName] = podIP
		}

		// Ignore metrics if cannot infer IP address of the origin pod.
		if podIP == "" {
			continue
		}

		// Don't invoke any k8s client functionality in passthrough mode.
		// Just tag the IP and forward the batch.
		if kp.passthroughMode {
			continue
		}

		// Add k8s tags to resource.
		attrsToAdd := kp.getAttributesForPodIP(podIP)
		if len(attrsToAdd) == 0 {
			continue
		}

		for k, v := range attrsToAdd {
			md.Resource.Labels[k] = v
		}
	}

	return kp.nextMetricsConsumer.ConsumeMetrics(ctx, metrics)
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
