// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/metrics"

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

type correctnessTestCase struct {
	t         *testing.T
	sender    testbed.DataSender
	receiver  testbed.DataReceiver
	harness   *testHarness
	collector testbed.OtelcolRunner
}

func newCorrectnessTestCase(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	harness *testHarness,
) *correctnessTestCase {
	return &correctnessTestCase{t: t, sender: sender, receiver: receiver, harness: harness}
}

func (tc *correctnessTestCase) startCollector() {
	tc.collector = testbed.NewInProcessCollector(componentFactories(tc.t))
	_, err := tc.collector.PrepareConfig(tc.t, correctnesstests.CreateConfigYaml(tc.t, tc.sender, tc.receiver, nil, nil))
	require.NoError(tc.t, err)
	rd, err := newResultsDir(tc.t.Name())
	require.NoError(tc.t, err)
	err = rd.mkDir()
	require.NoError(tc.t, err)
	fname, err := rd.fullPath("agent.log")
	require.NoError(tc.t, err)
	log.Println("starting collector")
	err = tc.collector.Start(testbed.StartParams{
		Name:        "Agent",
		LogFilePath: fname,
	})
	require.NoError(tc.t, err)
}

func (tc *correctnessTestCase) stopCollector() {
	_, err := tc.collector.Stop()
	require.NoError(tc.t, err)
}

func (tc *correctnessTestCase) startTestbedSender() {
	log.Println("starting testbed sender")
	err := tc.sender.Start()
	require.NoError(tc.t, err)
}

func (tc *correctnessTestCase) startTestbedReceiver() {
	log.Println("starting testbed receiver")
	err := tc.receiver.Start(&testbed.MockTraceConsumer{}, tc.harness, &testbed.MockLogConsumer{})
	require.NoError(tc.t, err)
}

func (tc *correctnessTestCase) stopTestbedReceiver() {
	log.Println("stopping testbed receiver")
	err := tc.receiver.Stop()
	require.NoError(tc.t, err)
}

func (tc *correctnessTestCase) sendFirstMetric() {
	tc.harness.sendNextMetric()
}

func (tc *correctnessTestCase) waitForAllMetrics() {
	log.Println("waiting for allMetricsReceived")
	for {
		select {
		case <-time.After(10 * time.Second):
			tc.t.Fatal("Deadline exceeded while waiting to receive metrics")
			return
		case <-tc.harness.allMetricsReceived:
			log.Println("all metrics received")
			return
		}
	}
}

func componentFactories(t *testing.T) otelcol.Factories {
	factories, err := testbed.Components()
	require.NoError(t, err)
	return factories
}
