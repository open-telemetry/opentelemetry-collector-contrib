// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"context"
	"runtime"
	"testing"
	"time"

	dockerclient "github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
)

func HostEndpoint(t *testing.T) string {
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	client, err := dockerclient.New(dockerclient.FromEnv)
	require.NoError(t, err)
	_, err = client.Ping(t.Context(), dockerclient.PingOptions{
		NegotiateAPIVersion: true,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", dockerclient.NetworkInspectOptions{})
	require.NoError(t, err)

	// Prefer IPv4 gateways, but fallback to IPv6 if no IPv4 gateway is found.
	// IPv6 addresses are wrapped in brackets so that callers can safely append
	// ":port" (e.g. [fc00:f853:ccd:e793::1]:4317).
	var ipv6Fallback string
	for _, ipam := range network.Network.IPAM.Config {
		if ipam.Gateway.String() == "" {
			continue
		}
		if ipam.Gateway.Is4() {
			return ipam.Gateway.String()
		}
		if ipv6Fallback == "" {
			ipv6Fallback = "[" + ipam.Gateway.String() + "]"
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
