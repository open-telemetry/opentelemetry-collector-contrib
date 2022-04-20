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

//go:build windows
// +build windows

package activedirectorydsreceiver

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/stretchr/testify/require"
)

func TestGetWatchers(t *testing.T) {
	creater := &mockCounterCreater{
		availableCounterNames: getAvailableCounters(t),
	}

	watchers, err := getWatchers(creater)
	require.NoError(t, err)
	require.NotNil(t, watchers)
}

func getAvailableCounters(t *testing.T) []string {
	prefix := fmt.Sprintf(`\%s(*)\`, object)

	f, err := ioutil.ReadFile(filepath.Join("testdata", "counters.txt"))
	require.NoError(t, err)

	lines := regexp.MustCompile("\r?\n").Split(string(f), -1)

	linesOut := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			linesOut = append(linesOut, strings.TrimPrefix(line, prefix))
		}
	}

	return linesOut
}

type mockCounterCreater struct {
	created               int
	availableCounterNames []string
}

func (m *mockCounterCreater) Create(counterName string) (winperfcounters.PerfCounterWatcher, error) {
	for _, availableCounter := range m.availableCounterNames {
		if counterName == availableCounter {
			watcher := &mockPerfCounterWatcher{
				val: float64(m.created),
			}

			m.created++

			return watcher, nil
		}
	}

	return nil, fmt.Errorf("counter %s is not available\navailable counters:\n\t%s", counterName, strings.Join(m.availableCounterNames, "\n\t"))
}
