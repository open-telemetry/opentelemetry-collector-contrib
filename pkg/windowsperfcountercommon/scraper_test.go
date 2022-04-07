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

package windowsperfcountercommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/windowsperfcountercommon"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func Test_PathBuilder(t *testing.T) {
	testCases := []struct {
		name          string
		cfgs          []ScraperCfg
		expectedErr   string
		expectedPaths []string
	}{
		{
			name: "basicPath",
			cfgs: []ScraperCfg{
				{
					CounterCfg: PerfCounterConfig{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Committed Bytes", Metric: "Committed Bytes"}},
					},
				},
			},
			expectedPaths: []string{},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			core, obs := observer.New(zapcore.WarnLevel)
			logger := zap.New(core)

			scrapers := BuildPaths(test.cfgs, logger)

			if test.expectedErr != "" {
				require.Equal(t, 1, obs.Len())
				log := obs.All()[0]
				require.EqualError(t, log.Context[0].Interface.(error), test.expectedErr)
				return
			}

			actualPaths := []string{}
			for _, scraper := range scrapers {
				actualPaths = append(actualPaths, scraper.Counter.Path())
			}

			require.Equal(t, test.expectedPaths, actualPaths)
		})
	}
}
