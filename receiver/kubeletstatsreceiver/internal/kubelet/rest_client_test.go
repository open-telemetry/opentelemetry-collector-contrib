// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"testing"

	"github.com/stretchr/testify/require"

	kube "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"
)

func TestRestClient(t *testing.T) {
	rest := NewRestClient(&fakeClient{})
	resp, _ := rest.StatsSummary()
	require.Equal(t, "/stats/summary", string(resp))
	resp, _ = rest.Pods()
	require.Equal(t, "/pods", string(resp))
}

var _ kube.Client = (*fakeClient)(nil)

type fakeClient struct{}

func (f *fakeClient) Get(path string) ([]byte, error) {
	return []byte(path), nil
}
