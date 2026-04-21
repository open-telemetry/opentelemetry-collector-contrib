// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
)

func TestMarshalEncoder_Metrics(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario   string
		encoding   string
		batchSize  int
		recordSize int

		validEncoder   bool
		count          int
		expectedError  bool
		expectedChunks int
	}{
		{
			scenario:     "Invalid encoding type provided",
			encoding:     "invalid-encoding",
			batchSize:    10,
			recordSize:   1000,
			validEncoder: false,
		},
		{
			scenario:      "valid jaeger encoder, does not implement metrics",
			encoding:      "jaeger_proto",
			batchSize:     10,
			recordSize:    1000,
			count:         10,
			validEncoder:  true,
			expectedError: true,
		},
		{
			scenario:      "valid zipkin proto encoder, does not implement metrics",
			encoding:      "zipkin_proto",
			batchSize:     10,
			recordSize:    1000,
			count:         10,
			validEncoder:  true,
			expectedError: true,
		},
		{
			scenario:      "valid zipkin JSON encoder, does not implement metrics",
			encoding:      "zipkin_json",
			batchSize:     10,
			recordSize:    1000,
			count:         10,
			validEncoder:  true,
			expectedError: true,
		},
		{
			scenario:       "valid otlp proto encoder that implements metrics",
			encoding:       "otlp_proto",
			batchSize:      10,
			recordSize:     100000,
			validEncoder:   true,
			count:          20,
			expectedError:  false,
			expectedChunks: 2,
		},
		{
			scenario:       "valid otlp JSON encoder that implements metrics",
			encoding:       "otlp_json",
			batchSize:      10,
			recordSize:     100000,
			validEncoder:   true,
			count:          20,
			expectedError:  false,
			expectedChunks: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.scenario, func(t *testing.T) {
			encoder, err := batch.NewEncoder(
				tc.encoding,
				batch.WithMaxRecordSize(tc.recordSize),
				batch.WithMaxRecordsPerBatch(tc.batchSize),
			)
			if !tc.validEncoder {
				require.ErrorIs(t, err, batch.ErrUnknownExportEncoder, "Must return the expected error when an invalid encoding is provided")
				return
			}
			require.NotNil(t, encoder, "Must have a valid encoder in order to proceed")

			bt, err := encoder.Metrics(NewTestMetrics(tc.count))
			if tc.expectedError {
				assert.ErrorIs(t, err, batch.ErrUnsupportedEncoding, "Must match the expected error value")
				return
			}
			assert.NoError(t, err, "Must not have return an error processing data")
			require.NotNil(t, bt, "Must have a valid batch")

			assert.Len(t, bt.Chunk(), tc.expectedChunks, "Must have provided the expected chunk amount")
		})
	}
}

func TestMarshalEncoder_Traces(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario   string
		encoding   string
		batchSize  int
		recordSize int

		validEncoder   bool
		count          int
		expectedChunks int
	}{
		{
			scenario:     "Invalid encoding type provided",
			encoding:     "invalid-encoding",
			batchSize:    10,
			recordSize:   1000,
			validEncoder: false,
		},
		{
			scenario:       "valid jaeger encoder",
			encoding:       "jaeger_proto",
			batchSize:      10,
			recordSize:     1000,
			count:          10,
			validEncoder:   true,
			expectedChunks: 1,
		},
		{
			scenario:       "valid zipkin proto encoder",
			encoding:       "zipkin_proto",
			batchSize:      10,
			recordSize:     1000,
			count:          10,
			validEncoder:   true,
			expectedChunks: 1,
		},
		{
			scenario:       "valid zipkin JSON encoder",
			encoding:       "zipkin_json",
			batchSize:      10,
			recordSize:     1000,
			count:          10,
			validEncoder:   true,
			expectedChunks: 1,
		},
		{
			scenario:       "valid otlp proto encoder",
			encoding:       "otlp_proto",
			batchSize:      10,
			recordSize:     100000,
			validEncoder:   true,
			count:          20,
			expectedChunks: 2,
		},
		{
			scenario:       "valid otlp JSON encoder",
			encoding:       "otlp_json",
			batchSize:      10,
			recordSize:     100000,
			validEncoder:   true,
			count:          20,
			expectedChunks: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.scenario, func(t *testing.T) {
			encoder, err := batch.NewEncoder(
				tc.encoding,
				batch.WithMaxRecordSize(tc.recordSize),
				batch.WithMaxRecordsPerBatch(tc.batchSize),
			)
			if !tc.validEncoder {
				require.ErrorIs(t, err, batch.ErrUnknownExportEncoder, "Must return the expected error when an invalid encoding is provided")
				return
			}
			require.NotNil(t, encoder, "Must have a valid encoder in order to proceed")

			bt, err := encoder.Traces(NewTestTraces(tc.count))
			assert.NoError(t, err, "Must not have return an error processing data")
			require.NotNil(t, bt, "Must have a valid batch")

			assert.Len(t, bt.Chunk(), tc.expectedChunks, "Must have provided the expected chunk amount")
		})
	}
}

func TestMarshalEncoder_Logs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario   string
		encoding   string
		batchSize  int
		recordSize int

		validEncoder   bool
		count          int
		expectedError  bool
		expectedChunks int
	}{
		{
			scenario:     "Invalid encoding type provided",
			encoding:     "invalid-encoding",
			batchSize:    10,
			recordSize:   1000,
			validEncoder: false,
		},
		{
			scenario:      "valid jaeger encoder, does not implement logs",
			encoding:      "jaeger_proto",
			batchSize:     10,
			recordSize:    1000,
			count:         10,
			validEncoder:  true,
			expectedError: true,
		},
		{
			scenario:      "valid zipkin proto encoder, does not implement logs",
			encoding:      "zipkin_proto",
			batchSize:     10,
			recordSize:    1000,
			count:         10,
			validEncoder:  true,
			expectedError: true,
		},
		{
			scenario:      "valid zipkin JSON encoder, does not implement logs",
			encoding:      "zipkin_json",
			batchSize:     10,
			recordSize:    1000,
			count:         10,
			validEncoder:  true,
			expectedError: true,
		},
		{
			scenario:       "valid otlp proto encoder that implements logs",
			encoding:       "otlp_proto",
			batchSize:      10,
			recordSize:     100000,
			validEncoder:   true,
			count:          20,
			expectedError:  false,
			expectedChunks: 2,
		},
		{
			scenario:       "valid otlp JSON encoder that implements logs",
			encoding:       "otlp_json",
			batchSize:      10,
			recordSize:     100000,
			validEncoder:   true,
			count:          20,
			expectedError:  false,
			expectedChunks: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.scenario, func(t *testing.T) {
			encoder, err := batch.NewEncoder(
				tc.encoding,
				batch.WithMaxRecordSize(tc.recordSize),
				batch.WithMaxRecordsPerBatch(tc.batchSize),
			)
			if !tc.validEncoder {
				require.ErrorIs(t, err, batch.ErrUnknownExportEncoder, "Must return the expected error when an invalid encoding is provided")
				return
			}
			require.NotNil(t, encoder, "Must have a valid encoder in order to proceed")

			bt, err := encoder.Logs(NewTestLogs(tc.count))
			if tc.expectedError {
				assert.ErrorIs(t, err, batch.ErrUnsupportedEncoding, "Must match the expected error value")
				return
			}
			assert.NoError(t, err, "Must not have return an error processing data")
			require.NotNil(t, bt, "Must have a valid batch")

			assert.Len(t, bt.Chunk(), tc.expectedChunks, "Must have provided the expected chunk amount")
		})
	}
}
