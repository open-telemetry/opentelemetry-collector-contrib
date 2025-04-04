// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package tests

// This file contains Test functions which initiate the tests. The tests can be either
// coded in this file or use scenarios from perf_scenarios.go.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	idutils "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// TestMain is used to initiate setup, execution and tear down of testbed.
func TestMain(m *testing.M) {
	testbed.DoTestMain(m, performanceResultsSummary)
}

func TestTrace10kSPS(t *testing.T) {
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		resourceSpec testbed.ResourceSpec
	}{
		{
			"OpenCensus",
			datasenders.NewOCTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			datareceivers.NewOCDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 39,
				ExpectedMaxRAM: 100,
			},
		},
		{
			"OTLP-gRPC",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 20,
				ExpectedMaxRAM: 100,
			},
		},
		{
			"OTLP-gRPC-gzip",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)).WithCompression("gzip"),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 30,
				ExpectedMaxRAM: 100,
			},
		},
		{
			"OTLP-HTTP",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), ""),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 20,
				ExpectedMaxRAM: 100,
			},
		},
		{
			"OTLP-HTTP-gzip",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), "gzip"),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)).WithCompression("gzip"),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 25,
				ExpectedMaxRAM: 100,
			},
		},
		{
			"OTLP-HTTP-zstd",
			testbed.NewOTLPHTTPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t), "zstd"),
			testbed.NewOTLPHTTPDataReceiver(testutil.GetAvailablePort(t)).WithCompression("zstd"),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 22,
				ExpectedMaxRAM: 220,
			},
		},
		{
			"SAPM",
			datasenders.NewSapmDataSender(testutil.GetAvailablePort(t), ""),
			datareceivers.NewSapmDataReceiver(testutil.GetAvailablePort(t), ""),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 32,
				ExpectedMaxRAM: 100,
			},
		},
		{
			"SAPM-gzip",
			datasenders.NewSapmDataSender(testutil.GetAvailablePort(t), "gzip"),
			datareceivers.NewSapmDataReceiver(testutil.GetAvailablePort(t), "gzip"),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 35,
				ExpectedMaxRAM: 110,
			},
		},
		{
			"SAPM-zstd",
			datasenders.NewSapmDataSender(testutil.GetAvailablePort(t), "zstd"),
			datareceivers.NewSapmDataReceiver(testutil.GetAvailablePort(t), "zstd"),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 32,
				ExpectedMaxRAM: 300,
			},
		},
		{
			"Zipkin",
			datasenders.NewZipkinDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			datareceivers.NewZipkinDataReceiver(testutil.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 120,
			},
		},
	}

	processors := []ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Scenario10kItemsPerSecond(
				t,
				test.sender,
				test.receiver,
				test.resourceSpec,
				performanceResultsSummary,
				processors,
				nil,
				nil,
			)
		})
	}
}

func TestTrace10kSPSJaegerGRPC(t *testing.T) {
	port := testutil.GetAvailablePort(t)
	receiver := datareceivers.NewJaegerDataReceiver(port)
	Scenario10kItemsPerSecondAlternateBackend(
		t,
		datasenders.NewJaegerGRPCDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
		receiver,
		testbed.NewOTLPDataReceiver(port),
		testbed.ResourceSpec{
			ExpectedMaxCPU: 40,
			ExpectedMaxRAM: 100,
		},
		performanceResultsSummary,
		[]ProcessorNameAndConfigBody{
			{
				Name: "batch",
				Body: `
  batch:
`,
			},
		},
		nil,
	)
}

func TestTraceNoBackend10kSPS(t *testing.T) {
	limitProcessors := []ProcessorNameAndConfigBody{
		{
			Name: "memory_limiter",
			Body: `
  memory_limiter:
   check_interval: 100ms
   limit_mib: 20
`,
		},
	}

	noLimitProcessors := []ProcessorNameAndConfigBody{}

	processorsConfig := []processorConfig{
		{
			Name:                "NoMemoryLimit",
			Processor:           noLimitProcessors,
			ExpectedMaxRAM:      100,
			ExpectedMinFinalRAM: 80,
		},
		{
			Name:                "MemoryLimit",
			Processor:           limitProcessors,
			ExpectedMaxRAM:      95,
			ExpectedMinFinalRAM: 50,
		},
	}

	for _, testConf := range processorsConfig {
		t.Run(testConf.Name, func(t *testing.T) {
			ScenarioTestTraceNoBackend10kSPS(
				t,
				testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
				testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
				testbed.ResourceSpec{ExpectedMaxCPU: 90, ExpectedMaxRAM: testConf.ExpectedMaxRAM},
				performanceResultsSummary,
				testConf,
			)
		})
	}
}

func TestTrace1kSPSWithAttrs(t *testing.T) {
	Scenario1kSPSWithAttrs(t, []string{}, []TestCase{
		// No attributes.
		{
			attrCount:      0,
			attrSizeByte:   0,
			expectedMaxCPU: 30,
			expectedMaxRAM: 150,
			resultsSummary: performanceResultsSummary,
		},

		// We generate 10 attributes each with average key length of 100 bytes and
		// average value length of 50 bytes so total size of attributes values is
		// 15000 bytes.
		{
			attrCount:      100,
			attrSizeByte:   50,
			expectedMaxCPU: 120,
			expectedMaxRAM: 150,
			resultsSummary: performanceResultsSummary,
		},

		// Approx 10 KiB attributes.
		{
			attrCount:      10,
			attrSizeByte:   1000,
			expectedMaxCPU: 100,
			expectedMaxRAM: 150,
			resultsSummary: performanceResultsSummary,
		},

		// Approx 100 KiB attributes.
		{
			attrCount:      20,
			attrSizeByte:   5000,
			expectedMaxCPU: 250,
			expectedMaxRAM: 150,
			resultsSummary: performanceResultsSummary,
		},
	}, nil, nil)
}

// verifySingleSpan sends a single span to Collector, waits until the span is forwarded
// and received by MockBackend and calls user-supplied verification functions on
// received span.
// Temporarily, we need two verification functions in order to verify spans in
// new and old format received by MockBackend.
func verifySingleSpan(
	t *testing.T,
	tc *testbed.TestCase,
	serviceName string,
	spanName string,
	verifyReceived func(span ptrace.Span),
) {
	// Clear previously received traces.
	tc.MockBackend.ClearReceivedItems()
	startCounter := tc.MockBackend.DataItemsReceived()

	// Send one span.
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName)
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(idutils.UInt64ToTraceID(0, 1))
	span.SetSpanID(idutils.UInt64ToSpanID(1))
	span.SetName(spanName)

	sender := tc.LoadGenerator.(*testbed.ProviderSender).Sender.(testbed.TraceDataSender)
	require.NoError(t, sender.ConsumeTraces(context.Background(), td))

	// We bypass the load generator in this test, but make sure to increment the
	// counter since it is used in final reports.
	tc.LoadGenerator.IncDataItemsSent()

	// Wait until span is received.
	tc.WaitFor(func() bool { return tc.MockBackend.DataItemsReceived() == startCounter+1 },
		"span received")

	// Verify received span.
	count := 0
	for _, td := range tc.MockBackend.ReceivedTraces {
		rs := td.ResourceSpans()
		for i := 0; i < rs.Len(); i++ {
			ils := rs.At(i).ScopeSpans()
			for j := 0; j < ils.Len(); j++ {
				spans := ils.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					verifyReceived(spans.At(k))
					count++
				}
			}
		}
	}
	assert.EqualValues(t, 1, count, "must receive one span")
}

func TestTraceAttributesProcessor(t *testing.T) {
	tests := []struct {
		name     string
		sender   testbed.DataSender
		receiver testbed.DataReceiver
	}{
		{
			"OTLP",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
			require.NoError(t, err)

			// Use processor to add attributes to certain spans.
			processors := []ProcessorNameAndConfigBody{
				{
					Name: "batch",
					Body: `
  batch:
`,
				},
				{
					Name: "attributes",
					Body: `
  attributes:
    include:
      match_type: regexp
      services: ["service-to-add.*"]
      span_names: ["span-to-add-.*"]
    actions:
      - action: insert
        key: "new_attr"
        value: "string value"
`,
				},
			}

			agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
			configStr := createConfigYaml(t, test.sender, test.receiver, resultDir, processors, nil)
			configCleanup, err := agentProc.PrepareConfig(t, configStr)
			require.NoError(t, err)
			defer configCleanup()

			options := testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10}
			dataProvider := testbed.NewPerfTestDataProvider(options)
			tc := testbed.NewTestCase(
				t,
				dataProvider,
				test.sender,
				test.receiver,
				agentProc,
				&testbed.PerfTestValidator{},
				performanceResultsSummary,
			)
			defer tc.Stop()

			tc.StartBackend()
			tc.StartAgent()
			defer tc.StopAgent()

			tc.EnableRecording()

			require.NoError(t, test.sender.Start())

			// Create a span that matches "include" filter.
			spanToInclude := "span-to-add-attr"
			// Create a service name that matches "include" filter.
			nodeToInclude := "service-to-add-attr"

			// verifySpan verifies that attributes was added to the internal data span.
			verifySpan := func(span ptrace.Span) {
				require.NotNil(t, span)
				require.Equal(t, 1, span.Attributes().Len())
				attrVal, ok := span.Attributes().Get("new_attr")
				assert.True(t, ok)
				assert.EqualValues(t, "string value", attrVal.Str())
			}

			verifySingleSpan(t, tc, nodeToInclude, spanToInclude, verifySpan)

			// Create a service name that does not match "include" filter.
			nodeToExclude := "service-not-to-add-attr"

			verifySingleSpan(t, tc, nodeToExclude, spanToInclude, func(span ptrace.Span) {
				// Verify attributes was not added to the new internal data span.
				assert.Equal(t, 0, span.Attributes().Len())
			})

			// Create another span that does not match "include" filter.
			spanToExclude := "span-not-to-add-attr"
			verifySingleSpan(t, tc, nodeToInclude, spanToExclude, func(span ptrace.Span) {
				// Verify attributes was not added to the new internal data span.
				assert.Equal(t, 0, span.Attributes().Len())
			})
		})
	}
}

func TestTraceAttributesProcessorJaegerGRPC(t *testing.T) {
	port := testutil.GetAvailablePort(t)
	sender := datasenders.NewJaegerGRPCDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := datareceivers.NewJaegerDataReceiver(port)
	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	// Use processor to add attributes to certain spans.
	processors := []ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
`,
		},
		{
			Name: "attributes",
			Body: `
  attributes:
    include:
      match_type: regexp
      services: ["service-to-add.*"]
      span_names: ["span-to-add-.*"]
    actions:
      - action: insert
        key: "new_attr"
        value: "string value"
`,
		},
	}

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, nil)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()

	options := testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10}
	dataProvider := testbed.NewPerfTestDataProvider(options)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		performanceResultsSummary,
	)
	defer tc.Stop()

	tc.MockBackend = testbed.NewMockBackend(tc.ComposeTestResultFileName("backend.log"), testbed.NewOTLPDataReceiver(port))

	tc.StartBackend()
	tc.StartAgent()
	defer tc.StopAgent()

	tc.EnableRecording()

	require.NoError(t, sender.Start())

	// Create a span that matches "include" filter.
	spanToInclude := "span-to-add-attr"
	// Create a service name that matches "include" filter.
	nodeToInclude := "service-to-add-attr"

	// verifySpan verifies that attributes was added to the internal data span.
	verifySpan := func(span ptrace.Span) {
		require.NotNil(t, span)
		require.Equal(t, 1, span.Attributes().Len())
		attrVal, ok := span.Attributes().Get("new_attr")
		assert.True(t, ok)
		assert.EqualValues(t, "string value", attrVal.Str())
	}

	verifySingleSpan(t, tc, nodeToInclude, spanToInclude, verifySpan)

	// Create a service name that does not match "include" filter.
	nodeToExclude := "service-not-to-add-attr"

	verifySingleSpan(t, tc, nodeToExclude, spanToInclude, func(span ptrace.Span) {
		// Verify attributes was not added to the new internal data span.
		assert.Equal(t, 0, span.Attributes().Len())
	})

	// Create another span that does not match "include" filter.
	spanToExclude := "span-not-to-add-attr"
	verifySingleSpan(t, tc, nodeToInclude, spanToExclude, func(span ptrace.Span) {
		// Verify attributes was not added to the new internal data span.
		assert.Equal(t, 0, span.Attributes().Len())
	})
}
