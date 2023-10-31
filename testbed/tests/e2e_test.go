// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package tests

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestIdleMode(t *testing.T) {

	options := testbed.LoadOptions{DataItemsPerSecond: 10_000, ItemsPerBatch: 10}
	dataProvider := testbed.NewPerfTestDataProvider(options)

	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t))
	cfg := createConfigYaml(t, sender, receiver, resultDir, nil, nil)
	cp := testbed.NewChildProcessCollector()
	cleanup, err := cp.PrepareConfig(cfg)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		cp,
		&testbed.PerfTestValidator{},
		performanceResultsSummary,
		testbed.WithResourceLimits(testbed.ResourceSpec{ExpectedMaxCPU: 20, ExpectedMaxRAM: 83}),
	)
	tc.StartAgent()

	tc.Sleep(tc.Duration)

	tc.Stop()
}

const ballastConfig = `
  memory_ballast:
    size_mib: %d
`

func TestBallastMemory(t *testing.T) {
	tests := []struct {
		ballastSize uint32
		maxRSS      uint32
	}{
		{100, 80},
		{500, 110},
		{1000, 120},
	}

	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	options := testbed.LoadOptions{DataItemsPerSecond: 10_000, ItemsPerBatch: 10}
	dataProvider := testbed.NewPerfTestDataProvider(options)
	for _, test := range tests {
		t.Run(fmt.Sprintf("ballast-size-%d", test.ballastSize), func(t *testing.T) {
			sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
			receiver := testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t))
			ballastCfg := createConfigYaml(
				t, sender, receiver, resultDir, nil,
				map[string]string{"memory_ballast": fmt.Sprintf(ballastConfig, test.ballastSize)})
			cp := testbed.NewChildProcessCollector()
			cleanup, err := cp.PrepareConfig(ballastCfg)
			require.NoError(t, err)
			tc := testbed.NewTestCase(
				t,
				dataProvider,
				sender,
				receiver,
				cp,
				&testbed.PerfTestValidator{},
				performanceResultsSummary,
				testbed.WithSkipResults(),
				testbed.WithResourceLimits(
					testbed.ResourceSpec{
						ExpectedMaxRAM:         test.maxRSS,
						ResourceCheckPeriod:    time.Second,
						MaxConsecutiveFailures: 5,
					},
				),
			)
			tc.StartAgent()

			var rss, vms uint32
			// It is possible that the process is not ready or the ballast code path
			// is not hit immediately so we give the process up to a couple of seconds
			// to fire up and setup ballast. 2 seconds is a long time for this case but
			// it is short enough to not be annoying if the test fails repeatedly
			tc.WaitForN(func() bool {
				rss, vms, _ = tc.AgentMemoryInfo()
				return vms > test.ballastSize
			}, time.Second*5, fmt.Sprintf("VMS must be greater than %d", test.ballastSize))

			// https://github.com/open-telemetry/opentelemetry-collector/issues/3233
			// given that the maxRSS isn't an absolute maximum and that the actual maximum might be a bit off,
			// we give some room here instead of failing when the memory usage isn't that much higher than the max
			lenientMax := 1.1 * float32(test.maxRSS)

			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/6927#issuecomment-1138624098
			// During garbage collection, we may observe the ballast in rss.
			// If this happens, adjust the baseline expectation for RSS size and validate that additional memory is
			// still within the expected limit.
			garbageCollectionMax := lenientMax + float32(test.ballastSize)

			rssTooHigh := fmt.Sprintf("The RSS memory usage (%d) is >10%% higher than the limit (%d).", rss, test.maxRSS)

			if rss > test.ballastSize {
				assert.LessOrEqual(t, float32(rss), garbageCollectionMax, rssTooHigh)
			} else {
				assert.LessOrEqual(t, float32(rss), lenientMax, rssTooHigh)
			}

			cleanup()
			tc.Stop()
		})
	}
}
