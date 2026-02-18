// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package gateway provides utilities for querying Kubernetes EndpointSlice information
// for a gateway collector deployment using the Kubernetes API.
package gateway // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/gateway"

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"
)

// EndpointSliceLister lists EndpointSlices for a given namespace and label selector.
// This interface allows tests to inject a fake implementation.
type EndpointSliceLister interface {
	ListEndpointSlices(ctx context.Context, namespace, labelSelector string) ([]discoveryv1.EndpointSlice, error)
}

// k8sLister is the real implementation backed by a Kubernetes client.
type k8sLister struct {
	client kubernetes.Interface
}

// ListEndpointSlices queries the Kubernetes API for EndpointSlices matching the given label selector.
func (l *k8sLister) ListEndpointSlices(ctx context.Context, namespace, labelSelector string) ([]discoveryv1.EndpointSlice, error) {
	list, err := l.client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// NewK8sLister creates an EndpointSliceLister backed by a Kubernetes client.
// It tries in-cluster config first, then falls back to the default kubeconfig.
func NewK8sLister() (EndpointSliceLister, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig (e.g. for local development or out-of-cluster deployments)
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
		cfg, err = kubeconfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
		}
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	return &k8sLister{client: client}, nil
}

// FetchGatewayInfo queries the Kubernetes EndpointSlice(s) that back the given
// service and returns a populated GatewayInfo.
//
// service may be in "name" or "namespace/name" format. When no namespace is
// provided, FetchGatewayInfo attempts to determine the current namespace from
// the in-cluster service account file.
func FetchGatewayInfo(ctx context.Context, service string, lister EndpointSliceLister) (payload.GatewayInfo, error) {
	svcName, namespace := parseServiceName(service)

	if namespace == "" {
		if ns, err := getInClusterNamespace(); err == nil {
			namespace = ns
		}
	}

	labelSelector := "kubernetes.io/service-name=" + svcName
	slices, err := lister.ListEndpointSlices(ctx, namespace, labelSelector)
	if err != nil {
		return payload.GatewayInfo{}, fmt.Errorf("failed to list endpoint slices for service %q: %w", service, err)
	}

	return buildGatewayInfo(svcName, namespace, slices), nil
}

// parseServiceName splits "namespace/service" into its components.
// If no slash is present the namespace is returned as an empty string.
func parseServiceName(service string) (name, namespace string) {
	if idx := strings.IndexByte(service, '/'); idx != -1 {
		return service[idx+1:], service[:idx]
	}
	return service, ""
}

const inClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func getInClusterNamespace() (string, error) {
	if _, err := os.Stat(inClusterNamespacePath); os.IsNotExist(err) {
		return "", fmt.Errorf("not running in-cluster, namespace not determinable")
	} else if err != nil {
		return "", fmt.Errorf("error checking namespace file: %w", err)
	}
	ns, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return "", fmt.Errorf("error reading namespace file: %w", err)
	}
	return string(ns), nil
}

func buildGatewayInfo(svcName, namespace string, slices []discoveryv1.EndpointSlice) payload.GatewayInfo {
	info := payload.GatewayInfo{
		Service:   svcName,
		Namespace: namespace,
	}

	portsSet := make(map[string]struct{})
	podsSet := make(map[string]struct{})
	addressesSet := make(map[string]struct{})

	for _, slice := range slices {
		if info.AddressType == "" {
			info.AddressType = string(slice.AddressType)
		}

		for _, p := range slice.Ports {
			if p.Port == nil {
				continue
			}
			portStr := fmt.Sprintf("%d", *p.Port)
			if p.Protocol != nil && string(*p.Protocol) != "" {
				portStr += "/" + string(*p.Protocol)
			}
			portsSet[portStr] = struct{}{}
		}

		for _, ep := range slice.Endpoints {
			if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" {
				podsSet[ep.TargetRef.Name] = struct{}{}
			}
			for _, addr := range ep.Addresses {
				addressesSet[addr] = struct{}{}
			}
		}
	}

	for p := range portsSet {
		info.Ports = append(info.Ports, p)
	}
	for pod := range podsSet {
		info.Pods = append(info.Pods, pod)
	}
	for addr := range addressesSet {
		info.Addresses = append(info.Addresses, addr)
	}

	// Sort for deterministic output.
	sort.Strings(info.Ports)
	sort.Strings(info.Pods)
	sort.Strings(info.Addresses)

	return info
}
