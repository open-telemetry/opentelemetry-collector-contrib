// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package k8swindows

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

// TestK8sWindowsWithoutHostName verify k8sWindows return error when HOST_NAME not set.
func TestK8sWindowsWithoutHostName(t *testing.T) {
	t.Setenv("HOST_NAME", "")
	k, err := New(zaptest.NewLogger(t), &stores.K8sDecorator{}, host.Info{})
	assert.Nil(t, k)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "missing environment variable HOST_NAME")
}

// TestK8sWindowsWithoutHostName verify k8sWindows can be initialized.
func TestK8sWindows(t *testing.T) {
	t.Setenv("HOST_NAME", "192.168.0.1")
	k, err := New(zaptest.NewLogger(t), &stores.K8sDecorator{}, host.Info{})
	assert.NotNil(t, k)
	assert.NoError(t, err)
}
