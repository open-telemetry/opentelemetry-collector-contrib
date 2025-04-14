// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"errors"

	"github.com/shirou/gopsutil/v4/common"
)

func validateRootPath(rootPath string) error {
	if rootPath == "" {
		return nil
	}
	return errors.New("root_path is supported on linux only")
}

func setGoPsutilEnvVars(_ string) common.EnvMap {
	return common.EnvMap{}
}
