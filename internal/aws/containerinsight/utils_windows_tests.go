// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package containerinsight // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostProcessContainer(t *testing.T) {
	t.Setenv(RunInContainer, "True")
	assert.False(t, IsWindowsHostProcessContainer())

	t.Setenv(RunAsHostProcessContainer, "True")
	assert.True(t, IsWindowsHostProcessContainer())
}
