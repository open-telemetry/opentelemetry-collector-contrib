// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package winperfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCounterPath(t *testing.T) {
	testCases := []struct {
		name         string
		object       string
		instance     string
		counterName  string
		expectedPath string
	}{
		{
			name:         "basicPath",
			object:       "Memory",
			counterName:  "Committed Bytes",
			expectedPath: "\\Memory\\Committed Bytes",
		},
		{
			name:         "basicPathWithInstance",
			object:       "Web Service",
			instance:     "_Total",
			counterName:  "Current Connections",
			expectedPath: "\\Web Service(_Total)\\Current Connections",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			path := counterPath(test.object, test.instance, test.counterName)
			require.Equal(t, test.expectedPath, path)
		})
	}
}

// Test_Scraping_Wildcard tests that wildcard instances pull out values
func Test_Scraping_Wildcard(t *testing.T) {
	watcher, err := NewWatcher("LogicalDisk", "*", "Free Megabytes")
	require.NoError(t, err)

	values, err := watcher.ScrapeData()
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(values), 3)
}
