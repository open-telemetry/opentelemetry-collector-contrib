// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewPodmanConnectionUnsupported(t *testing.T) {
	logger := zap.NewNop()
	c, err := newPodmanConnection(logger, "xyz://hello", "", "")
	assert.EqualError(t, err, `unable to create connection. "xyz" is not a supported schema`)
	assert.Nil(t, c)
}

func TestNewPodmanConnectionUnix(t *testing.T) {
	tmpDir := t.TempDir()

	socketPath := filepath.Join(tmpDir, "test.sock")
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	logger := zap.NewNop()
	c, err := newPodmanConnection(logger, "unix:///"+socketPath, "", "")
	assert.NoError(t, err)
	assert.NotNil(t, c)

	tr, ok := c.Transport.(*http.Transport)
	assert.True(t, ok)
	assert.True(t, tr.DisableCompression)
	conn, err := tr.DialContext(context.Background(), "", "")
	assert.NoError(t, err)
	assert.Equal(t, socketPath, conn.RemoteAddr().String())
}

func TestNewPodmanConnectionSSH(t *testing.T) {
	// We only test that the connection tries to connect over SSH.
	// Actual SSH connection to podman should be tested in an integration test if desired.
	logger := zap.NewNop()
	c, err := newPodmanConnection(logger, "ssh://otel-test-podman-server", "", "")
	assert.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "connection to bastion host (ssh://otel-test-podman-server) failed:"))
	assert.Nil(t, c)
}
