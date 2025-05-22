// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package gopsutilenv // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"

import (
	"context"
	"errors"

	"github.com/shirou/gopsutil/v4/common"
)

func ValidateRootPath(rootPath string) error {
	if rootPath == "" {
		return nil
	}
	return errors.New("root_path is supported on linux only")
}

func SetGoPsutilEnvVars(_ string) common.EnvMap {
	return common.EnvMap{}
}

func GetEnvWithContext(_ context.Context, _ string, dfault string, _ ...string) string {
	return dfault
}

func SetGlobalRootPath(_ string) {
}
