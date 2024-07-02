// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestCreateNewTracesReceiver(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)

	tests := []struct {
		desc     string
		config   Config
		consumer consumer.Traces
		err      error
	}{
		{
			desc:     "Default config succeeds",
			config:   *defaultConfig,
			consumer: consumertest.NewNop(),
			err:      nil,
		},
		{
			desc: "User defined config success",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "localhost:8080",
				},
				Secret: "mysecret",
			},
			consumer: consumertest.NewNop(),
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			rec, err := newTracesReceiver(receivertest.NewNopCreateSettings(), &test.config, test.consumer)
			if test.err == nil {
				require.NotNil(t, rec)
			} else {
				require.ErrorIs(t, err, test.err)
				require.Nil(t, rec)
			}
		})
	}
}

func TestEventToTracesTraces(t *testing.T) {
	tests := []struct {
		desc            string
		payloadFilePath string
		eventType       string
		expectedError   error
		expectedSpans   int
	}{
		{
			desc:            "WorkflowJobEvent processing",
			payloadFilePath: "./testdata/completed/5_workflow_job_completed.json",
			eventType:       "workflow_job",
			expectedError:   nil,
			expectedSpans:   10, // 10 spans in the payload
		},
		{
			desc:            "WorkflowRunEvent processing",
			payloadFilePath: "./testdata/completed/8_workflow_run_completed.json",
			eventType:       "workflow_run",
			expectedError:   nil,
			expectedSpans:   1, // Root span
		},
	}

	logger := zaptest.NewLogger(t)
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			payload, err := os.ReadFile(test.payloadFilePath)
			require.NoError(t, err)

			event, err := github.ParseWebHook(test.eventType, payload)
			require.NoError(t, err)

			traces, err := eventToTraces(event, &Config{}, logger)

			if test.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, test.expectedError, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, test.expectedSpans, traces.SpanCount(), fmt.Sprintf("%s: unexpected number of spans", test.desc))
		})
	}
}

func TestProcessSteps(t *testing.T) {
	tests := []struct {
		desc             string
		givenSteps       []*github.TaskStep
		expectedSpans    int
		expectedStatuses []ptrace.StatusCode
	}{
		{
			desc: "Multiple steps with mixed status",

			givenSteps: []*github.TaskStep{
				{Name: getPtr("Checkout"), Status: getPtr("completed"), Conclusion: getPtr("success")},
				{Name: getPtr("Build"), Status: getPtr("completed"), Conclusion: getPtr("failure")},
				{Name: getPtr("Test"), Status: getPtr("completed"), Conclusion: getPtr("success")},
			},
			expectedSpans: 4, // Includes parent span
			expectedStatuses: []ptrace.StatusCode{
				ptrace.StatusCodeOk,
				ptrace.StatusCodeError,
				ptrace.StatusCodeOk,
			},
		},
		{
			desc:             "No steps",
			givenSteps:       []*github.TaskStep{},
			expectedSpans:    1, // Only the parent span should be created
			expectedStatuses: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logger := zap.NewNop()
			traces := ptrace.NewTraces()
			rs := traces.ResourceSpans().AppendEmpty()
			ss := rs.ScopeSpans().AppendEmpty()

			traceID, _ := generateTraceID(123, 1)
			parentSpanID := createParentSpan(ss, tc.givenSteps, &github.WorkflowJob{}, traceID, logger)

			processSteps(ss, tc.givenSteps, &github.WorkflowJob{}, traceID, parentSpanID, logger)

			startIdx := 1 // Skip the parent span if it's the first one
			if len(tc.expectedStatuses) == 0 {
				startIdx = 0 // No steps, only the parent span exists
			}

			require.Equal(t, tc.expectedSpans, ss.Spans().Len(), "Unexpected number of spans")
			for i, expectedStatusCode := range tc.expectedStatuses {
				span := ss.Spans().At(i + startIdx)
				statusCode := span.Status().Code()
				require.Equal(t, expectedStatusCode, statusCode, fmt.Sprintf("Unexpected status code for span #%d", i+startIdx))
			}
		})
	}
}

func TestResourceAndSpanAttributesCreation(t *testing.T) {
	tests := []struct {
		desc            string
		payloadFilePath string
		expectedSteps   []map[string]string
	}{
		{
			desc:            "WorkflowJobEvent Step Attributes",
			payloadFilePath: "./testdata/completed/5_workflow_job_completed.json",
			expectedSteps: []map[string]string{
				{"ci.github.workflow.job.step.name": "Set up job", "ci.github.workflow.job.step.number": "1"},
				{"ci.github.workflow.job.step.name": "Run actions/checkout@v3", "ci.github.workflow.job.step.number": "2"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			logger := zaptest.NewLogger(t)

			payload, err := os.ReadFile(tc.payloadFilePath)
			require.NoError(t, err)

			event, err := github.ParseWebHook("workflow_job", payload)
			require.NoError(t, err)

			traces, err := eventToTraces(event, &Config{}, logger)
			require.NoError(t, err)

			rs := traces.ResourceSpans().At(0)
			ss := rs.ScopeSpans().At(0)

			for _, expectedStep := range tc.expectedSteps {
				stepFound := false

				for i := 0; i < ss.Spans().Len() && !stepFound; i++ {
					span := ss.Spans().At(i)
					attrs := span.Attributes()

					stepValue, found := attrs.Get("ci.github.workflow.job.step.name")
					stepName := stepValue.Str()

					if !found || stepName == "" { // Skip if the attribute is not found or name is empty
						continue
					}

					expectedStepName := expectedStep["ci.github.workflow.job.step.name"]

					if stepName == expectedStepName {
						stepFound = true
						for attrKey, expectedValue := range expectedStep {
							attrValue, found := attrs.Get(attrKey)
							if !found {
								require.Fail(t, fmt.Sprintf("Attribute '%s' not found in span for step '%s'", attrKey, stepName))
								continue
							}
							actualValue := attributeValueToString(attrValue)
							require.Equal(t, expectedValue, actualValue, "Attribute '%s' does not match expected value for step '%s'", attrKey, stepName)
						}
					}
				}

				require.True(t, stepFound, "Step '%s' not found in any span", expectedStep["ci.github.workflow.job.step.name"])
			}

		})
	}
}

// attributeValueToString converts an attribute value to a string regardless of its actual type
func attributeValueToString(attr pcommon.Value) string {
	switch attr.Type() {
	case pcommon.ValueTypeStr:
		return attr.Str()
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(attr.Int(), 10)
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(attr.Double(), 'f', -1, 64)
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(attr.Bool())
	case pcommon.ValueTypeMap:
		return "<Map Value>"
	case pcommon.ValueTypeSlice:
		return "<Slice Value>"
	default:
		return "<Unknown Value Type>"
	}
}

func getPtr(str string) *string {
	return &str
}
