// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package activedirectorydsreceiver

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

func TestGetWatchers(t *testing.T) {
	c := &mockCounterCreater{
		availableCounterNames: getAvailableCounters(t),
	}

	watchers, err := getWatchers(c)
	require.NoError(t, err)
	require.NotNil(t, watchers)
}

func getAvailableCounters(t *testing.T) []string {
	prefix := fmt.Sprintf(`\%s(*)\`, object)

	f, err := os.ReadFile(filepath.Join("testdata", "counters.txt"))
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
