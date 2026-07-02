// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	api_v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	kubeletPodsPath                  = "/pods"
	defaultKubeletPodsRequestTimeout = 30 * time.Second
)

type kubeletPodLister struct {
	client   *http.Client
	endpoint string
}

func newKubeletPodLister(apiCfg k8sconfig.APIConfig, cfg KubeletConfig, node string) (*kubeletPodLister, error) {
	endpoint := cfg.Endpoint
	requestTimeout := cfg.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = defaultKubeletPodsRequestTimeout
	}
	if endpoint == "" {
		endpoint = "https://" + net.JoinHostPort(node, "10250") + kubeletPodsPath
	}
	if !strings.HasSuffix(endpoint, kubeletPodsPath) {
		endpoint = strings.TrimRight(endpoint, "/") + kubeletPodsPath
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	allowedScheme := parsedEndpoint.Scheme == "https" || (cfg.AllowInsecureHTTP && parsedEndpoint.Scheme == "http")
	if !allowedScheme {
		return nil, errors.New("kubelet.endpoint must use https unless allow_insecure_http is enabled")
	}

	restCfg, err := k8sconfig.CreateRestConfig(apiCfg)
	if err != nil {
		return nil, err
	}
	if cfg.InsecureSkipVerify {
		restCfg.Insecure = true
		restCfg.CAData = nil
		restCfg.CAFile = ""
	}
	transport, err := rest.TransportFor(restCfg)
	if err != nil {
		return nil, err
	}
	return &kubeletPodLister{
		client:   &http.Client{Transport: transport, Timeout: requestTimeout},
		endpoint: endpoint,
	}, nil
}

func (l *kubeletPodLister) listPods(ctx context.Context) (*api_v1.PodList, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("kubelet /pods returned %s", resp.Status)
	}

	var pods api_v1.PodList
	if err := json.NewDecoder(resp.Body).Decode(&pods); err != nil {
		return nil, err
	}
	return &pods, nil
}
