// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"context"
	"runtime"
	"strings"
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
	// Prefer IPv4 gateway over IPv6 to ensure compatibility with IPv4-only clusters
	for _, ipam := range network.IPAM.Config {
		if ipam.Gateway != "" && !isIPv6(ipam.Gateway) {
			return ipam.Gateway
		}
	}
	// Fallback to any gateway if no IPv4 found
	for _, ipam := range network.IPAM.Config {
		if ipam.Gateway != "" {
			return ipam.Gateway
		}
	}
	require.Fail(t, "failed to find host endpoint")
	return ""
}

func isIPv6(addr string) bool {
	// IPv6 addresses contain colons, IPv4 addresses do not
	return strings.Contains(addr, ":")
}

func SelectorFromMap(labelMap map[string]any) labels.Selector {
	labelStringMap := make(map[string]string)
	for key, value := range labelMap {
		labelStringMap[key] = value.(string)
	}
	labelSet := labels.Set(labelStringMap)
	return labelSet.AsSelector()
}
