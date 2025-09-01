// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package iisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver"

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func setupTestMain(m *testing.M) {
	if runtime.GOOS == "windows" && runtime.GOARCH != "arm64" && os.Getenv("GITHUB_ACTIONS") == "true" {
		// In this case it is necessary to install IIS for CI tests
		if err := exec.Command("powershell", "-Command", "Install-WindowsFeature -Name Web-Server -IncludeManagementTools").Run(); err != nil {
			panic(fmt.Errorf("failed to install IIS for tests: %w", err))
		}
	}

	os.Exit(m.Run())
}
