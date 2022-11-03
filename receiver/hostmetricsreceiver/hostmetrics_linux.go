// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package hostmetricsreceiver

import (
	"fmt"
	"os"
	"strings"
)

func validateRootPath(rootPath string) error {
	if rootPath == "" || rootPath == "/" {
		return nil
	}

	if _, err := os.Stat(rootPath); err != nil {
		return fmt.Errorf("invalid root_path: %w", err)
	}

	// Validate the rootPath is consistent with the gopsutil envvars.
	for _, envVarKey := range []string{
		"HOST_PROC",
		"HOST_SYS",
		"HOST_ETC",
		"HOST_VAR",
		"HOST_RUN",
		"HOST_DEV",
	} {
		envVarVal := os.Getenv(envVarKey)
		if envVarVal == "" {
			continue
		}
		if !strings.HasPrefix(envVarVal, rootPath) {
			return fmt.Errorf(
				"config `root_path=%s` is inconsistent with envvar `%s=%s` config, root_path must be the prefix of the environment variable",
				rootPath,
				envVarKey,
				envVarVal)
		}
	}
	return nil
}
