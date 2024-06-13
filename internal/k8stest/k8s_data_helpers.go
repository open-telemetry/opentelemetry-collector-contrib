// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
)

func HostEndpoint(t *testing.T) string {
	if runtime.GOOS == "darwin" {
		return "host.docker.internal"
	}

	host := os.Getenv("HOST_ENDPOINT")
	if host != "" {
		return host
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	client.NegotiateAPIVersion(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	network, err := client.NetworkInspect(ctx, "kind", types.NetworkInspectOptions{})
	require.NoError(t, err)

	for _, ipam := range network.IPAM.Config {
		return ipam.Gateway
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
