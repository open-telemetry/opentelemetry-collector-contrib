// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !windows

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/system"

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// keep as var for testing
var hostnamePath = "/bin/hostname"

func getSystemFQDN(ctx context.Context) (string, error) {
	// Go does not provide a way to get the full hostname
	// so we make a best-effort by running the hostname binary
	// if available
	if _, err := os.Stat(hostnamePath); err == nil {
		localCtx, cancel := context.WithTimeout(ctx, time.Second*1)
		defer cancel()
		cmd := exec.CommandContext(localCtx, hostnamePath, "-f")
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	// if stat failed for any reason, fail silently
	return "", nil
}
