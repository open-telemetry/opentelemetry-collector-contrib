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

type mockCounterCreater struct{
	created int
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
