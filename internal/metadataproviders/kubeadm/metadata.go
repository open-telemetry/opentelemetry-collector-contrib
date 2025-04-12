// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeadm // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/kubeadm"

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

type Provider interface {
	// ClusterName returns the current K8S cluster name
	ClusterName(ctx context.Context) (string, error)
	// ClusterUID returns the current K8S cluster UID
	ClusterUID(ctx context.Context) (string, error)
}

type LocalCache struct {
	ClusterName string
	ClusterUID  string
}

type kubeadmProvider struct {
	kubeadmClient       kubernetes.Interface
	configMapName       string
	kubeSystemNamespace string
	cache               LocalCache
}

func NewProvider(configMapName string, kubeSystemNamespace string, apiConf k8sconfig.APIConfig) (Provider, error) {
	k8sAPIClient, err := k8sconfig.MakeClient(apiConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s API client: %w", err)
	}
	return &kubeadmProvider{
		kubeadmClient:       k8sAPIClient,
		configMapName:       configMapName,
		kubeSystemNamespace: kubeSystemNamespace,
	}, nil
}

func (k *kubeadmProvider) ClusterName(ctx context.Context) (string, error) {
	if k.cache.ClusterName != "" {
		return k.cache.ClusterName, nil
	}
	configmap, err := k.kubeadmClient.CoreV1().ConfigMaps(k.kubeSystemNamespace).Get(ctx, k.configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch ConfigMap with name %s and namespace %s from K8s API: %w", k.configMapName, k.kubeSystemNamespace, err)
	}

	k.cache.ClusterName = configmap.Data["clusterName"]

	return k.cache.ClusterName, nil
}

func (k *kubeadmProvider) ClusterUID(ctx context.Context) (string, error) {
	if k.cache.ClusterUID != "" {
		return k.cache.ClusterUID, nil
	}
	ns, err := k.kubeadmClient.CoreV1().Namespaces().Get(ctx, k.kubeSystemNamespace, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch Namespace %s from K8s API: %w", k.kubeSystemNamespace, err)
	}

	k.cache.ClusterUID = string(ns.GetUID())

	return k.cache.ClusterUID, nil
}
