// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package kubelet

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

func TestSAPathInHostProcessContainer(t *testing.T) {
	// todo: Remove this workaround func when Windows AMIs has containerd 1.7 which solves upstream bug.

	// Test default SA cert and token.
	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", svcAcctCACertPath)
	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/token", svcAcctTokenPath)

	// Test SA cert and token when run inside container.
	t.Setenv(containerinsight.RunInContainer, "True")
	updateSVCPath()
	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", svcAcctCACertPath)
	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/token", svcAcctTokenPath)

	// Test SA cert and token when run inside host process container.
	t.Setenv(containerinsight.RunAsHostProcessContainer, "True")
	t.Setenv("CONTAINER_SANDBOX_MOUNT_POINT", "test123456")
	updateSVCPath()
	assert.Equal(t, "test123456/var/run/secrets/kubernetes.io/serviceaccount/ca.crt", svcAcctCACertPath)
	assert.Equal(t, "test123456/var/run/secrets/kubernetes.io/serviceaccount/token", svcAcctTokenPath)
}
