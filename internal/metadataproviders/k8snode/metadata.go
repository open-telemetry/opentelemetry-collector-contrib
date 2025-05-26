// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snode // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/k8snode"

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

type Provider interface {
	// NodeUID returns the K8S Node UID
	NodeUID(ctx context.Context) (string, error)
	// NodeName returns the current K8S Node Name
	NodeName(ctx context.Context) (string, error)
}

type k8snodeProvider struct {
	k8snodeClient kubernetes.Interface
	nodeName      string
}

func NewProvider(nodeName string, apiConf k8sconfig.APIConfig) (Provider, error) {
	if nodeName == "" {
		return nil, errors.New("nodeName can't be empty")
	}
	k8sAPIClient, err := k8sconfig.MakeClient(apiConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create K8s API client: %w", err)
	}
	return &k8snodeProvider{
		k8snodeClient: k8sAPIClient,
		nodeName:      nodeName,
	}, nil
}

func (k *k8snodeProvider) NodeUID(ctx context.Context) (string, error) {
	node, err := k.k8snodeClient.CoreV1().Nodes().Get(ctx, k.nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch node with name %s from K8s API: %w", k.nodeName, err)
	}
	return string(node.UID), nil
}

func (k *k8snodeProvider) NodeName(ctx context.Context) (string, error) {
	node, err := k.k8snodeClient.CoreV1().Nodes().Get(ctx, k.nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to fetch node with name %s from K8s API: %w", k.nodeName, err)
	}
	return node.Name, nil
}
