// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package gopsutilenv // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v4/common"
)

var gopsutilEnvVars = map[common.EnvKeyType]string{
	common.HostProcEnvKey: "/proc",
	common.HostSysEnvKey:  "/sys",
	common.HostEtcEnvKey:  "/etc",
	common.HostVarEnvKey:  "/var",
	common.HostRunEnvKey:  "/run",
	common.HostDevEnvKey:  "/dev",
}

// This exists to validate that different components that use this module do not
// have inconsistent root_path configurations. The root_path is passed down to gopsutil
// through context, so it must be consistent across the process.
var globalRootPath string

func ValidateRootPath(rootPath string) error {
	if rootPath == "" || rootPath == "/" {
		return nil
	}

	if globalRootPath != "" && rootPath != globalRootPath {
		return fmt.Errorf("inconsistent root_path configuration detected among components: `%s` != `%s`", globalRootPath, rootPath)
	}
	globalRootPath = rootPath

	if _, err := os.Stat(rootPath); err != nil {
		return fmt.Errorf("invalid root_path: %w", err)
	}

	return nil
}

func SetGoPsutilEnvVars(rootPath string) common.EnvMap {
	m := common.EnvMap{}
	if rootPath == "" || rootPath == "/" {
		return m
	}

	for envVarKey, defaultValue := range gopsutilEnvVars {
		_, ok := os.LookupEnv(string(envVarKey))
		if ok {
			continue // don't override if existing env var is set
		}
		m[envVarKey] = filepath.Join(rootPath, defaultValue)
	}
	return m
}

// SetGlobalRootPath mainly used for unit tests
func SetGlobalRootPath(rootPath string) {
	globalRootPath = rootPath
}

// copied from gopsutil:
// GetEnvWithContext retrieves the environment variable key. If it does not exist it returns the default.
// The context may optionally contain a map superseding os.EnvKey.
func GetEnvWithContext(ctx context.Context, key string, dfault string, combineWith ...string) string {
	var value string
	if env, ok := ctx.Value(common.EnvKey).(common.EnvMap); ok {
		value = env[common.EnvKeyType(key)]
	}
	if value == "" {
		value = os.Getenv(key)
	}
	if value == "" {
		value = dfault
	}

	return combine(value, combineWith)
}

func combine(value string, combineWith []string) string {
	switch len(combineWith) {
	case 0:
		return value
	case 1:
		return filepath.Join(value, combineWith[0])
	default:
		all := make([]string, len(combineWith)+1)
		all[0] = value
		copy(all[1:], combineWith)
		return filepath.Join(all...)
	}
}
