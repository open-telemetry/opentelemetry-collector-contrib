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

func (c fakeClient) delimiter() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}

	return "\n"
}

func (fakeClient) retrieveInfo() (string, error) {
	return readFile("info")
}

func (fakeClient) close() error {
	return nil
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
	require.Nil(t, err)
	require.True(t, strings.HasPrefix(res, "# Server"))
}
