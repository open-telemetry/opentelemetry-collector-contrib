// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"context"
	"net"
	"runtime"
	"testing"
	"time"

	network2 "github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
)

func HostEndpoint(t *testing.T) string {
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(t.Context())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", network2.InspectOptions{})
	require.NoError(t, err)

	// Prefer IPv4 gateways, but fallback to IPv6 if no IPv4 gateway is found.
	// IPv6 addresses are wrapped in brackets so that callers can safely append
	// ":port" (e.g. [fc00:f853:ccd:e793::1]:4317).
	var ipv6Fallback string
	for _, ipam := range network.IPAM.Config {
		if ipam.Gateway == "" {
			continue
		}
		ip := net.ParseIP(ipam.Gateway)
		if ip != nil && ip.To4() != nil {
			return ipam.Gateway
		}
		if ipv6Fallback == "" {
			ipv6Fallback = "[" + ipam.Gateway + "]"
		}
	}
	if ipv6Fallback != "" {
		return ipv6Fallback
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}

func SelectorFromMap(labelMap map[string]any) labels.Selector {
	labelStringMap := make(map[string]string)
	for key, value := range labelMap {
		labelStringMap[key] = value.(string)
	}
	labelSet := labels.Set(labelStringMap)
	return labelSet.AsSelector()
}
