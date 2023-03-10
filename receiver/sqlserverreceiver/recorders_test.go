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

//go:build windows
// +build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPerfCounterRecorders requires each perf counter record created to exist in the available counters.
func TestPerfCounterRecorders(t *testing.T) {
	expectedCounters := getAvailableCounters(t)

	for _, counter := range perfCounterRecorders {
		for counterPath := range counter.recorders {
			counterFullName := fmt.Sprintf("%s %s", counter.object, counterPath)
			_, ok := expectedCounters[counterFullName]
			require.True(t, ok, fmt.Sprintf("counter %s not found", counterFullName))
		}
	}
}

// getAvailableCounters populates a map containing all available counters.
func getAvailableCounters(t *testing.T) map[string]bool {
	file, err := os.Open(filepath.Join("testdata", "counters.txt"))
	require.NoError(t, err)

	defer file.Close()

	scanner := bufio.NewScanner(file)
	counterNames := map[string]bool{}
	for scanner.Scan() {
		if scanner.Text() != "" {
			nameNoBackslash := strings.ReplaceAll(scanner.Text(), "\\", " ")
			nameNoInstance := strings.TrimSpace(strings.ReplaceAll(nameNoBackslash, "(*)", ""))
			counterNames[nameNoInstance] = true
		}
	}

	return counterNames
}
