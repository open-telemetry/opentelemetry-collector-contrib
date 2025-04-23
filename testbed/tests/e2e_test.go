// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package tests

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestIdleMode(t *testing.T) {
	options := testbed.LoadOptions{DataItemsPerSecond: 10_000, ItemsPerBatch: 10}
	dataProvider := testbed.NewPerfTestDataProvider(options)

	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
	cfg := createConfigYaml(t, sender, receiver, resultDir, nil, nil)
	cp := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))

	cleanup, err := cp.PrepareConfig(t, cfg)
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
	t.Cleanup(tc.Stop)

	tc.Sleep(tc.Duration)
}
