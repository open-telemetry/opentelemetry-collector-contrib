// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"errors"
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

// erroringClusterInfoClient wraps fakeClient but fails CLUSTER INFO, to test that the
// receiver still returns the metrics derived from INFO when CLUSTER INFO is unavailable.
type erroringClusterInfoClient struct {
	fakeClient
}

func (erroringClusterInfoClient) retrieveClusterInfo() (string, error) {
	return "", errors.New("cluster info unavailable")
}

// erroringInfoClient wraps fakeClient but fails INFO, to test that the receiver surfaces
// that error rather than attempting to fall back to CLUSTER INFO alone.
type erroringInfoClient struct {
	fakeClient
}

func (erroringInfoClient) retrieveInfo() (string, error) {
	return "", errors.New("info unavailable")
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
