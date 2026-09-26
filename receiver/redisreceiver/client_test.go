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

var _ client = (*fakeClusterClient)(nil)

// A cluster-enabled instance. Same INFO as the standalone fixture except for the
// one flag that tells the scraper CLUSTER INFO is worth asking for.
type fakeClusterClient struct {
	fakeClient
}

func newFakeClusterClient() *fakeClusterClient {
	return &fakeClusterClient{}
}

func (fakeClusterClient) retrieveInfo() (string, error) {
	str, err := readFile("info")
	if err != nil {
		return "", err
	}
	return strings.Replace(str, "cluster_enabled:0", "cluster_enabled:1", 1), nil
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

func TestRetrieveClusterInfo(t *testing.T) {
	g := fakeClient{}
	res, err := g.retrieveClusterInfo()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(res, "cluster_enabled:1"))
}
