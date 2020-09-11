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

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor/kube"
)

const (
	k8sIPLabelName    string = "k8s.pod.ip"
	clientIPLabelName string = "ip"
	hostnameLabelName string = "host.hostname"
)

type kubernetesprocessor struct {
	logger          *zap.Logger
	apiConfig       k8sconfig.APIConfig
	kc              kube.Client
	passthroughMode bool
	rules           kube.ExtractionRules
	filters         kube.Filters
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

// ProcessTraces process traces and add k8s metadata using resource IP or incoming IP as pod origin.
func (kp *kubernetesprocessor) ProcessTraces(ctx context.Context, td pdata.Traces) (pdata.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		if rs.IsNil() {
			continue
		}

		_ = kp.processResource(ctx, rs.Resource(), false)
	}

	return td, nil
}

// ProcessMetrics process metrics and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) ProcessMetrics(ctx context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		ms := rm.At(i)
		if ms.IsNil() {
			continue
		}

		_ = kp.processResource(ctx, ms.Resource(), true)
	}

	return md, nil
}

// ProcessLogs process metrics and add k8s metadata using resource IP, hostname or incoming IP as pod origin.
func (kp *kubernetesprocessor) ProcessLogs(ctx context.Context, ld pdata.Logs) (pdata.Logs, error) {
	lm := ld.ResourceLogs()
	for i := 0; i < lm.Len(); i++ {
		ls := lm.At(i)
		if ls.IsNil() {
			continue
		}

		_ = kp.processResource(ctx, ls.Resource(), true)
	}

	return ld, nil
}

func (kp *kubernetesprocessor) processResource(ctx context.Context, resource pdata.Resource, checkHostname bool) error {
	var podIP string

	// check if the application, a collector/agent or a prior processor has already
	// annotated the batch with IP.
	if !resource.IsNil() {
		podIP = kp.k8sIPFromAttributes(resource.Attributes())
	}

	// Most of the metric receivers uses "host.hostname" resource label to identify metrics
	// origin. In k8s environment, it's set to a pod IP address. If the value doesn't represent
	// an IP address, we skip it.
	if podIP == "" && !resource.IsNil() && checkHostname {
		podIP = kp.k8sIPFromHostnameAttributes(resource.Attributes())
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
		return nil
	}

	// add k8s tags to resource
	attrsToAdd := kp.getAttributesForPodIP(podIP)
	if len(attrsToAdd) == 0 {
		return nil
	}

	if resource.IsNil() {
		resource.InitEmpty()
	}

	attrs := resource.Attributes()
	for k, v := range attrsToAdd {
		attrs.InsertString(k, v)
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

func (kp *kubernetesprocessor) k8sIPFromAttributes(attrs pdata.AttributeMap) string {
	ip := stringAttributeFromMap(attrs, k8sIPLabelName)
	if ip == "" {
		ip = stringAttributeFromMap(attrs, clientIPLabelName)
	}
	return ip
}

func (kp *kubernetesprocessor) k8sIPFromHostnameAttributes(attrs pdata.AttributeMap) string {
	hostname := stringAttributeFromMap(attrs, hostnameLabelName)
	if net.ParseIP(hostname) != nil {
		return hostname
	}
	return ""
}

func stringAttributeFromMap(attrs pdata.AttributeMap, key string) string {
	if val, ok := attrs.Get(key); ok {
		if val.Type() == pdata.AttributeValueSTRING {
			return val.StringVal()
		}
	}
	return ""
}
