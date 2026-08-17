// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"

	"go.uber.org/zap"
	api_v1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
)

const (
	kubeletPodsPath = "/pods"
)

type kubeletPodLister struct {
	client kubelet.Client
}

func newKubeletPodLister(apiCfg k8sconfig.APIConfig, cfg KubeletConfig, node string, logger *zap.Logger) (*kubeletPodLister, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		if apiCfg.AuthType == k8sconfig.AuthTypeKubeConfig {
			endpoint = node
		} else {
			endpoint = "https://" + net.JoinHostPort(node, "10250")
		}
	}
	endpoint = strings.TrimRight(endpoint, "/")
	if apiCfg.AuthType == k8sconfig.AuthTypeKubeConfig {
		if strings.Contains(endpoint, "://") || strings.Contains(endpoint, "/") {
			return nil, errors.New("kubelet.endpoint must be a node name when auth_type is kubeConfig")
		}
	} else {
		endpoint = strings.TrimSuffix(endpoint, kubeletPodsPath)
		if strings.Contains(endpoint, "://") {
			parsedEndpoint, err := url.Parse(endpoint)
			if err != nil {
				return nil, err
			}
			allowedScheme := parsedEndpoint.Scheme == "https" || (cfg.AllowInsecureHTTP && parsedEndpoint.Scheme == "http")
			if !allowedScheme {
				return nil, errors.New("kubelet.endpoint must use https unless allow_insecure_http is enabled")
			}
		} else if apiCfg.AuthType == k8sconfig.AuthTypeNone && !cfg.AllowInsecureHTTP {
			return nil, errors.New("kubelet.endpoint must use https unless allow_insecure_http is enabled")
		}
	}

	provider, err := kubelet.NewClientProvider(endpoint, &kubelet.ClientConfig{
		APIConfig:          apiCfg,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}, logger)
	if err != nil {
		return nil, err
	}
	client, err := provider.BuildClient()
	if err != nil {
		return nil, err
	}
	return &kubeletPodLister{client: client}, nil
}

func (l *kubeletPodLister) listPods(ctx context.Context) (*api_v1.PodList, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := l.client.Get(kubeletPodsPath)
	if err != nil {
		return nil, err
	}
	var pods api_v1.PodList
	if err := json.Unmarshal(body, &pods); err != nil {
		return nil, err
	}
	return &pods, nil
}
