// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ client = (*fakeClient)(nil)

type fakeClient struct{}

func newFakeClient() *fakeClient {
	return &fakeClient{}
}

func (fakeClient) delimiter() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}

	return "\n"
}

func (fakeClient) retrieveInfo() (string, error) {
	return readFile("info")
}

func (fakeClient) retrieveClusterInfo() (string, error) {
	return readFile("cluster_info")
}

func (fakeClient) close() error {
	return nil
}

// standaloneClient wraps fakeClient but reports cluster mode disabled, and tracks whether
// retrieveClusterInfo was called, to test that CLUSTER INFO is never attempted against a
// standalone (non-cluster) server.
type standaloneClient struct {
	fakeClient
	clusterInfoCalled bool
}

func (standaloneClient) retrieveInfo() (string, error) {
	return "cluster_enabled:0", nil
}

func (c *standaloneClient) retrieveClusterInfo() (string, error) {
	c.clusterInfoCalled = true
	return readFile("cluster_info")
}

func readFile(fname string) (string, error) {
	file, err := os.ReadFile(filepath.Join("testdata", fname+".txt"))
	if err != nil {
		return "", err
	}
	return string(file), nil
}

func TestRetrieveInfo(t *testing.T) {
	g := fakeClient{}
	res, err := g.retrieveInfo()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(res, "# Server"))
}
