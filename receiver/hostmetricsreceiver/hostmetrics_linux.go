// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"fmt"
	"os"
	"path/filepath"
)

var gopsutilEnvVars = map[string]string{
	"HOST_PROC": "/proc",
	"HOST_SYS":  "/sys",
	"HOST_ETC":  "/etc",
	"HOST_VAR":  "/var",
	"HOST_RUN":  "/run",
	"HOST_DEV":  "/dev",
}

// This exists to validate that different instances of the hostmetricsreceiver do not
// have inconsistent root_path configurations. The root_path is passed down to gopsutil
// through env vars, so it must be consistent across the process.
var globalRootPath string

func validateRootPath(rootPath string, env environment) error {
	if rootPath == "" || rootPath == "/" {
		return nil
	}

	if globalRootPath != "" && rootPath != globalRootPath {
		return fmt.Errorf("inconsistent root_path configuration detected between hostmetricsreceivers: `%s` != `%s`", globalRootPath, rootPath)
	}
	globalRootPath = rootPath

	if _, err := os.Stat(rootPath); err != nil {
		return fmt.Errorf("invalid root_path: %w", err)
	}

	return nil
}

func setGoPsutilEnvVars(rootPath string, env environment) error {
	if rootPath == "" || rootPath == "/" {
		return nil
	}

	for envVarKey, defaultValue := range gopsutilEnvVars {
		_, ok := env.Lookup(envVarKey)
		if ok {
			continue // don't override if existing env var is set
		}
		if err := env.Set(envVarKey, filepath.Join(rootPath, defaultValue)); err != nil {
			return err
		}
	}
	return nil
}
