// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package kubelet

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"

	"github.com/stretchr/testify/assert"
)

func TestSAPathInHostProcessContainer(t *testing.T) {
	// todo: Remove this workaround func when Windows AMIs has containerd 1.7 which solves upstream bug.

	// Test default SA cert and token.
	assert.Equal(t, svcAcctCACertPath, "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	assert.Equal(t, svcAcctTokenPath, "/var/run/secrets/kubernetes.io/serviceaccount/token")

	// Test SA cert and token when run inside container.
	t.Setenv(containerinsight.RunInContainer, "True")
	updateSVCPath()
	assert.Equal(t, svcAcctCACertPath, "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	assert.Equal(t, svcAcctTokenPath, "/var/run/secrets/kubernetes.io/serviceaccount/token")

	// Test SA cert and token when run inside host process container.
	t.Setenv(containerinsight.RunAsHostProcessContainer, "True")
	t.Setenv("CONTAINER_SANDBOX_MOUNT_POINT", "test123456")
	updateSVCPath()
	assert.Equal(t, svcAcctCACertPath, "test123456/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	assert.Equal(t, svcAcctTokenPath, "test123456/var/run/secrets/kubernetes.io/serviceaccount/token")
}
