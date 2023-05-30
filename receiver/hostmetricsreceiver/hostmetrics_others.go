// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import "fmt"

func validateRootPath(rootPath string, _ environment) error {
	if rootPath == "" {
		return nil
	}
	return fmt.Errorf("root_path is supported on linux only")
}

func setGoPsutilEnvVars(_ string, _ environment) error {
	return nil
}
