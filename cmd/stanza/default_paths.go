// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var agentName = "stanza"

func defaultPluginDir() string {
	if stat, err := os.Stat("./plugins"); err == nil {
		if stat.IsDir() {
			return "./plugins"
		}
	}

	return filepath.Join(agentHome(), "plugins")
}

func defaultConfig() string {
	if _, err := os.Stat("./config.yaml"); err == nil {
		return "./config.yaml"
	}

	return filepath.Join(agentHome(), "config.yaml")
}

func agentHome() string {
	if home := os.Getenv(strings.ToUpper(agentName) + "_HOME"); home != "" {
		return home
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(`C:\`, agentName)
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, agentName)
	case "linux":
		return filepath.Join("/opt", agentName)
	default:
		panic(fmt.Sprintf("Unsupported GOOS %s", runtime.GOOS))
	}
}
