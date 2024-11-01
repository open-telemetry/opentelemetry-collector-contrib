// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	framework "k8s.io/client-go/tools/cache/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	serviceHostEnv = "KUBERNETES_SERVICE_HOST"
	servicePortEnv = "KUBERNETES_SERVICE_PORT"
)

func mockServiceHost(tb testing.TB, c *Config) {
	c.AuthType = k8sconfig.AuthTypeNone
	tb.Setenv(serviceHostEnv, "mock")
	tb.Setenv(servicePortEnv, "12345")
}

func TestNewExtension(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	mockServiceHost(t, config)

	ext, err := newObserver(config, extensiontest.NewNopSettings())
	require.NoError(t, err)
	require.NotNil(t, ext)
}

func TestExtensionObserveServices(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.ObservePods = false // avoid causing data race when multiple test cases running in the same process using podListerWatcher
	mockServiceHost(t, config)

	set := extensiontest.NewNopSettings()
	set.ID = component.NewID(metadata.Type)
	ext, err := newObserver(config, set)
	require.NoError(t, err)
	require.NotNil(t, ext)

	obs := ext.(*k8sObserver)
	serviceListerWatcher := framework.NewFakeControllerSource()
	obs.serviceListerWatcher = serviceListerWatcher

	serviceListerWatcher.Add(serviceWithClusterIP)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	sink := &endpointSink{}
	obs.ListAndWatch(sink)

	requireSink(t, sink, func() bool {
		return len(sink.added) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/service-1-UID",
		Target: "service-1.default.svc.cluster.local",
		Details: &observer.K8sService{
			Name:      "service-1",
			Namespace: "default",
			UID:       "service-1-UID",
			Labels: map[string]string{
				"env": "prod",
			},
			ClusterIP:   "1.2.3.4",
			ServiceType: "ClusterIP",
		},
	}, sink.added[0])

	serviceListerWatcher.Modify(serviceWithClusterIPV2)

	requireSink(t, sink, func() bool {
		return len(sink.changed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/service-1-UID",
		Target: "service-1.default.svc.cluster.local",
		Details: &observer.K8sService{
			Name:      "service-1",
			Namespace: "default",
			UID:       "service-1-UID",
			Labels: map[string]string{
				"env":             "prod",
				"service-version": "2",
			},
			ClusterIP:   "1.2.3.4",
			ServiceType: "ClusterIP",
		},
	}, sink.changed[0])

	serviceListerWatcher.Delete(serviceWithClusterIPV2)

	requireSink(t, sink, func() bool {
		return len(sink.removed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/service-1-UID",
		Target: "service-1.default.svc.cluster.local",
		Details: &observer.K8sService{
			Name:      "service-1",
			Namespace: "default",
			UID:       "service-1-UID",
			Labels: map[string]string{
				"env":             "prod",
				"service-version": "2",
			},
			ClusterIP:   "1.2.3.4",
			ServiceType: "ClusterIP",
		},
	}, sink.removed[0])

	require.NoError(t, ext.Shutdown(context.Background()))
	obs.StopListAndWatch()
}

func TestExtensionObservePods(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	mockServiceHost(t, config)

	set := extensiontest.NewNopSettings()
	set.ID = component.NewID(metadata.Type)
	ext, err := newObserver(config, set)
	require.NoError(t, err)
	require.NotNil(t, ext)

	obs := ext.(*k8sObserver)
	podListerWatcher := framework.NewFakeControllerSource()
	obs.podListerWatcher = podListerWatcher

	podListerWatcher.Add(pod1V1)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	sink := &endpointSink{}
	obs.ListAndWatch(sink)

	requireSink(t, sink, func() bool {
		return len(sink.added) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/pod1-UID",
		Target: "1.2.3.4",
		Details: &observer.Pod{
			Name:      "pod1",
			Namespace: "default",
			UID:       "pod1-UID",
			Labels: map[string]string{
				"env": "prod",
			},
		},
	}, sink.added[0])

	podListerWatcher.Modify(pod1V2)

	requireSink(t, sink, func() bool {
		return len(sink.changed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/pod1-UID",
		Target: "1.2.3.4",
		Details: &observer.Pod{
			Name:      "pod1",
			Namespace: "default",
			UID:       "pod1-UID",
			Labels: map[string]string{
				"env":         "prod",
				"pod-version": "2",
			},
		},
	}, sink.changed[0])

	podListerWatcher.Delete(pod1V2)

	requireSink(t, sink, func() bool {
		return len(sink.removed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/pod1-UID",
		Target: "1.2.3.4",
		Details: &observer.Pod{
			Name:      "pod1",
			Namespace: "default",
			UID:       "pod1-UID",
			Labels: map[string]string{
				"env":         "prod",
				"pod-version": "2",
			},
		},
	}, sink.removed[0])

	require.NoError(t, ext.Shutdown(context.Background()))
	obs.StopListAndWatch()
}

func TestExtensionObserveNodes(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.ObservePods = false // avoid causing data race when multiple test cases running in the same process using podListerWatcher
	mockServiceHost(t, config)

	set := extensiontest.NewNopSettings()
	set.ID = component.NewID(metadata.Type)
	ext, err := newObserver(config, set)
	require.NoError(t, err)
	require.NotNil(t, ext)

	obs := ext.(*k8sObserver)
	nodeListerWatcher := framework.NewFakeControllerSource()
	obs.nodeListerWatcher = nodeListerWatcher

	nodeListerWatcher.Add(node1V1)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	sink := &endpointSink{}
	obs.ListAndWatch(sink)

	requireSink(t, sink, func() bool {
		return len(sink.added) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/node1-uid",
		Target: "internalIP",
		Details: &observer.K8sNode{
			UID:                 "uid",
			Annotations:         map[string]string{"annotation-key": "annotation-value"},
			Labels:              map[string]string{"label-key": "label-value"},
			Name:                "node1",
			InternalIP:          "internalIP",
			InternalDNS:         "internalDNS",
			Hostname:            "localhost",
			ExternalIP:          "externalIP",
			ExternalDNS:         "externalDNS",
			KubeletEndpointPort: 1234,
		},
	}, sink.added[0])

	nodeListerWatcher.Modify(node1V2)

	requireSink(t, sink, func() bool {
		return len(sink.changed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/node1-uid",
		Target: "internalIP",
		Details: &observer.K8sNode{
			UID:         "uid",
			Annotations: map[string]string{"annotation-key": "annotation-value"},
			Labels: map[string]string{
				"label-key":    "label-value",
				"node-version": "2",
			},
			Name:                "node1",
			InternalIP:          "internalIP",
			InternalDNS:         "internalDNS",
			Hostname:            "localhost",
			ExternalIP:          "externalIP",
			ExternalDNS:         "externalDNS",
			KubeletEndpointPort: 1234,
		},
	}, sink.changed[0])

	nodeListerWatcher.Delete(node1V2)

	requireSink(t, sink, func() bool {
		return len(sink.removed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/node1-uid",
		Target: "internalIP",
		Details: &observer.K8sNode{
			UID:         "uid",
			Annotations: map[string]string{"annotation-key": "annotation-value"},
			Labels: map[string]string{
				"label-key":    "label-value",
				"node-version": "2",
			},
			Name:                "node1",
			InternalIP:          "internalIP",
			InternalDNS:         "internalDNS",
			Hostname:            "localhost",
			ExternalIP:          "externalIP",
			ExternalDNS:         "externalDNS",
			KubeletEndpointPort: 1234,
		},
	}, sink.removed[0])

	require.NoError(t, ext.Shutdown(context.Background()))
	obs.StopListAndWatch()
}

func TestExtensionObserveIngresses(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	config.ObservePods = false // avoid causing data race when multiple test cases running in the same process using podListerWatcher
	config.ObserveIngresses = true
	mockServiceHost(t, config)

	set := extensiontest.NewNopSettings()
	set.ID = component.NewID(metadata.Type)
	ext, err := newObserver(config, set)
	require.NoError(t, err)
	require.NotNil(t, ext)

	obs := ext.(*k8sObserver)
	ingressListerWatcher := framework.NewFakeControllerSource()
	obs.ingressListerWatcher = ingressListerWatcher

	ingressListerWatcher.Add(ingress)

	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	sink := &endpointSink{}
	obs.ListAndWatch(sink)

	requireSink(t, sink, func() bool {
		return len(sink.added) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/ingress-1-UID/host-1/",
		Target: "https://host-1/",
		Details: &observer.K8sIngress{
			Name:      "application-ingress",
			UID:       "k8s_observer/ingress-1-UID/host-1/",
			Labels:    map[string]string{"env": "prod"},
			Namespace: "default",
			Scheme:    "https",
			Host:      "host-1",
			Path:      "/",
		},
	}, sink.added[0])

	ingressListerWatcher.Modify(ingressV2)

	requireSink(t, sink, func() bool {
		return len(sink.changed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/ingress-1-UID/host-1/",
		Target: "https://host-1/",
		Details: &observer.K8sIngress{
			Name:      "application-ingress",
			UID:       "k8s_observer/ingress-1-UID/host-1/",
			Labels:    map[string]string{"env": "hardening"},
			Namespace: "default",
			Scheme:    "https",
			Host:      "host-1",
			Path:      "/",
		},
	}, sink.changed[0])

	ingressListerWatcher.Delete(ingressV2)

	requireSink(t, sink, func() bool {
		return len(sink.removed) == 1
	})

	assert.Equal(t, observer.Endpoint{
		ID:     "k8s_observer/ingress-1-UID/host-1/",
		Target: "https://host-1/",
		Details: &observer.K8sIngress{
			Name:      "application-ingress",
			UID:       "k8s_observer/ingress-1-UID/host-1/",
			Labels:    map[string]string{"env": "hardening"},
			Namespace: "default",
			Scheme:    "https",
			Host:      "host-1",
			Path:      "/",
		},
	}, sink.removed[0])

	require.NoError(t, ext.Shutdown(context.Background()))
}
