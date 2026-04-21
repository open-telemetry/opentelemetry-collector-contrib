// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
)

func TestTransform(t *testing.T) {
	trafficDist := "PreferSameZone"
	originalService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			UID:       "test-service-uid",
			Labels: map[string]string{
				"app": "test",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape":                             "true",
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1"}`,
				"kubernetes.io/config.source":                      "api",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"app": "test",
			},
			ClusterIP:                "10.0.0.1",
			PublishNotReadyAddresses: true,
			TrafficDistribution:      &trafficDist,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
				},
			},
		},
	}

	transformed := Transform(originalService)

	assert.Equal(t, "test-service", transformed.Name)
	assert.Equal(t, "default", transformed.Namespace)
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, transformed.Spec.Type)
	assert.Equal(t, map[string]string{"app": "test"}, transformed.Spec.Selector)
	assert.Equal(t, map[string]string{"app": "test"}, transformed.Labels)
	assert.True(t, transformed.Spec.PublishNotReadyAddresses)
	assert.NotNil(t, transformed.Spec.TrafficDistribution)
	assert.Equal(t, "PreferSameZone", *transformed.Spec.TrafficDistribution)
	assert.Len(t, transformed.Status.LoadBalancer.Ingress, 1)
	assert.Equal(t, "1.2.3.4", transformed.Status.LoadBalancer.Ingress[0].IP)

	assert.Contains(t, transformed.Annotations, "prometheus.io/scrape")
	assert.NotContains(t, transformed.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	assert.Contains(t, transformed.Annotations, "kubernetes.io/config.source")
	assert.Len(t, transformed.Annotations, 2)
}

func TestRecordMetrics(t *testing.T) {
	settings := receiver.Settings{}
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sServiceEndpointCount.Enabled = true
	mbc.ResourceAttributes.K8sServicePublishNotReadyAddresses.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			UID:       "test-service-uid",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	counts := EndpointCountsByKey{
		{AddressType: "IPv4", Zone: "us-east-1a"}: EndpointCounts{
			Ready:       3,
			Serving:     4,
			Terminating: 1,
		},
	}

	ts := pcommon.NewTimestampFromTime(time.Now())
	RecordMetrics(zap.NewNop(), mb, service, counts, ts)

	metrics := mb.Emit()
	require.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	resource := rm.Resource()

	serviceNameAttr, ok := resource.Attributes().Get("k8s.service.name")
	assert.True(t, ok)
	assert.Equal(t, "test-service", serviceNameAttr.Str())

	serviceUIDAttr, ok := resource.Attributes().Get("k8s.service.uid")
	assert.True(t, ok)
	assert.Equal(t, "test-service-uid", serviceUIDAttr.Str())

	serviceTypeAttr, ok := resource.Attributes().Get("k8s.service.type")
	assert.True(t, ok)
	assert.Equal(t, "ClusterIP", serviceTypeAttr.Str())

	namespaceAttr, ok := resource.Attributes().Get("k8s.namespace.name")
	assert.True(t, ok)
	assert.Equal(t, "default", namespaceAttr.Str())

	publishNotReadyAttr, ok := resource.Attributes().Get("k8s.service.publish_not_ready_addresses")
	assert.True(t, ok)
	assert.False(t, publishNotReadyAttr.Bool())

	assert.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	assert.Positive(t, sm.Metrics().Len())

	// Check for endpoint count metric with different conditions and address types
	foundReady := false
	foundServing := false
	foundTerminating := false

	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		if metric.Name() == "k8s.service.endpoint.count" {
			for j := 0; j < metric.Gauge().DataPoints().Len(); j++ {
				dp := metric.Gauge().DataPoints().At(j)

				conditionAttr, _ := dp.Attributes().Get("k8s.service.endpoint.condition")
				addressTypeAttr, _ := dp.Attributes().Get("k8s.service.endpoint.address_type")
				zoneAttr, _ := dp.Attributes().Get("k8s.service.endpoint.zone")

				assert.Equal(t, "IPv4", addressTypeAttr.Str())
				assert.Equal(t, "us-east-1a", zoneAttr.Str())

				switch conditionAttr.Str() {
				case "ready":
					foundReady = true
					assert.Equal(t, int64(3), dp.IntValue())
				case "serving":
					foundServing = true
					assert.Equal(t, int64(4), dp.IntValue())
				case "terminating":
					foundTerminating = true
					assert.Equal(t, int64(1), dp.IntValue())
				}
			}
		}
	}

	assert.True(t, foundReady, "Expected k8s.service.endpoint.count metric with condition=ready")
	assert.True(t, foundServing, "Expected k8s.service.endpoint.count metric with condition=serving")
	assert.True(t, foundTerminating, "Expected k8s.service.endpoint.count metric with condition=terminating")
}

func TestRecordMetrics_PublishNotReadyAddresses(t *testing.T) {
	settings := receiver.Settings{}
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sServiceEndpointCount.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service-publish-not-ready",
			Namespace: "default",
			UID:       "test-service-uid-publish-not-ready",
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeClusterIP,
			PublishNotReadyAddresses: true,
		},
	}

	counts := EndpointCountsByKey{
		{AddressType: "IPv4", Zone: "us-west-2a"}: EndpointCounts{
			Ready: 5,
		},
	}

	ts := pcommon.NewTimestampFromTime(time.Now())
	RecordMetrics(zap.NewNop(), mb, service, counts, ts)

	metrics := mb.Emit()
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)

	found := false
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		if metric.Name() == "k8s.service.endpoint.count" {
			for j := 0; j < metric.Gauge().DataPoints().Len(); j++ {
				dp := metric.Gauge().DataPoints().At(j)
				conditionAttr, _ := dp.Attributes().Get("k8s.service.endpoint.condition")
				if conditionAttr.Str() == "ready" {
					assert.Equal(t, int64(5), dp.IntValue())
					found = true
				}
			}
		}
	}
	assert.True(t, found)
}

func TestRecordMetrics_LoadBalancer(t *testing.T) {
	settings := receiver.Settings{}
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sServiceLoadBalancerIngressCount.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lb-service",
			Namespace: "default",
			UID:       "test-lb-service-uid",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "1.2.3.4"},
				},
			},
		},
	}

	counts := EndpointCountsByKey{
		{AddressType: "IPv4", Zone: ""}: EndpointCounts{
			Ready:       2,
			Serving:     2,
			Terminating: 0,
		},
	}

	ts := pcommon.NewTimestampFromTime(time.Now())
	RecordMetrics(zap.NewNop(), mb, service, counts, ts)

	metrics := mb.Emit()
	require.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)

	foundLBIngressCount := false
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		if metric.Name() == "k8s.service.load_balancer.ingress.count" {
			foundLBIngressCount = true
			assert.Equal(t, int64(1), metric.Gauge().DataPoints().At(0).IntValue())
		}
	}

	assert.True(t, foundLBIngressCount, "Expected k8s.service.load_balancer.ingress.count metric for LoadBalancer service")
}

func TestRecordMetrics_LoadBalancerNotReady(t *testing.T) {
	settings := receiver.Settings{}
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.K8sServiceLoadBalancerIngressCount.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-lb-service-pending",
			Namespace: "default",
			UID:       "test-lb-service-pending-uid",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{},
			},
		},
	}

	counts := EndpointCountsByKey{
		{AddressType: "IPv4", Zone: ""}: EndpointCounts{
			Ready:       0,
			Serving:     0,
			Terminating: 0,
		},
	}

	ts := pcommon.NewTimestampFromTime(time.Now())
	RecordMetrics(zap.NewNop(), mb, service, counts, ts)

	metrics := mb.Emit()
	require.Equal(t, 1, metrics.ResourceMetrics().Len())

	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)

	foundLBIngressCount := false
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		if metric.Name() == "k8s.service.load_balancer.ingress.count" {
			foundLBIngressCount = true
			assert.Equal(t, int64(0), metric.Gauge().DataPoints().At(0).IntValue())
		}
	}

	assert.True(t, foundLBIngressCount, "Expected k8s.service.load_balancer.ingress.count metric for LoadBalancer service")
}

func TestShouldSkipAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "skip kubectl last-applied-configuration",
			key:      "kubectl.kubernetes.io/last-applied-configuration",
			expected: true,
		},
		{
			name:     "allow control-plane annotations",
			key:      "control-plane.alpha.kubernetes.io/leader",
			expected: false,
		},
		{
			name:     "allow kubernetes.io/config annotations",
			key:      "kubernetes.io/config.seen",
			expected: false,
		},
		{
			name:     "allow prometheus annotations",
			key:      "prometheus.io/scrape",
			expected: false,
		},
		{
			name:     "allow custom annotations",
			key:      "my-company.io/team",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSkipAnnotation(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMetadata(t *testing.T) {
	trafficDist := "PreferSameZone"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
			UID:       "test-service-uid",
			Labels: map[string]string{
				"app":     "myapp",
				"version": "v1",
			},
			Annotations: map[string]string{
				"prometheus.io/scrape":                             "true",
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1"}`,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:                     corev1.ServiceTypeNodePort,
			PublishNotReadyAddresses: true,
			TrafficDistribution:      &trafficDist,
			Selector: map[string]string{
				"app": "myapp",
			},
		},
	}

	metadata := GetMetadata(Transform(service))

	require.Len(t, metadata, 1)
	require.Contains(t, metadata, experimentalmetricmetadata.ResourceID("test-service-uid"))

	km := metadata[experimentalmetricmetadata.ResourceID("test-service-uid")]
	assert.Equal(t, "k8s.service", km.EntityType)
	assert.Equal(t, "k8s.service.uid", km.ResourceIDKey)
	assert.Equal(t, experimentalmetricmetadata.ResourceID("test-service-uid"), km.ResourceID)

	assert.Equal(t, "test-service", km.Metadata["k8s.service.name"])
	assert.Equal(t, "default", km.Metadata["k8s.namespace.name"])
	assert.Contains(t, km.Metadata, "k8s.service.creation_timestamp")

	assert.Equal(t, "myapp", km.Metadata["k8s.service.selector.app"])
	assert.Equal(t, "myapp", km.Metadata["k8s.service.label.app"])
	assert.Equal(t, "v1", km.Metadata["k8s.service.label.version"])

	assert.Equal(t, "true", km.Metadata["k8s.service.annotation.prometheus.io/scrape"])
	assert.NotContains(t, km.Metadata, "k8s.service.annotation.kubectl.kubernetes.io/last-applied-configuration")
}
