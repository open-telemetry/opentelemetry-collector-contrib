// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"fmt"
	"os"

	"github.com/shirou/gopsutil/v3/common"
)

var gopsutilEnvVars = map[common.EnvKeyType]string{
	common.HostProcEnvKey: "/proc",
	common.HostSysEnvKey:  "/sys",
	common.HostEtcEnvKey:  "/etc",
	common.HostVarEnvKey:  "/var",
	common.HostRunEnvKey:  "/run",
	common.HostDevEnvKey:  "/dev",
}

// This exists to validate that different instances of the hostmetricsreceiver do not
// have inconsistent root_path configurations. The root_path is passed down to gopsutil
// through env vars, so it must be consistent across the process.
var globalRootPath string

func validateRootPath(rootPath string) error {
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

func setGoPsutilEnvVars(rootPath string, env environment) common.EnvMap {
	m := common.EnvMap{}
	if rootPath == "" || rootPath == "/" {
		return m
	}

	for envVarKey, defaultValue := range gopsutilEnvVars {
		_, ok := env.Lookup(string(envVarKey))
		if ok {
			continue // don't override if existing env var is set
		}
		m[envVarKey] = filepath.Join(rootPath, defaultValue)
	}
	return m
}
