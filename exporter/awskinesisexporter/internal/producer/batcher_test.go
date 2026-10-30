// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/producer"
)

type MockKinesisAPI struct {
	producer.Kinesis

	op     func(*kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error)
	descOp func(*kinesis.DescribeStreamInput) (*kinesis.DescribeStreamOutput, error)
}

func (mka *MockKinesisAPI) PutRecords(_ context.Context, r *kinesis.PutRecordsInput, _ ...func(*kinesis.Options)) (*kinesis.PutRecordsOutput, error) {
	if mka.op != nil {
		return mka.op(r)
	}
	return nil, nil
}

func (mka *MockKinesisAPI) DescribeStream(_ context.Context, r *kinesis.DescribeStreamInput, _ ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
	if mka.descOp != nil {
		return mka.descOp(r)
	}
	return nil, nil
}

func SetPutRecordsOperation(op func(r *kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error)) producer.Kinesis {
	return &MockKinesisAPI{op: op}
}

func SuccessfulPutRecordsOperation(_ *kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error) {
	return &kinesis.PutRecordsOutput{
		FailedRecordCount: aws.Int32(0),
		Records: []types.PutRecordsResultEntry{
			{ShardId: aws.String("0000000000000000000001"), SequenceNumber: aws.String("0000000000000000000001")},
		},
	}, nil
}

func HardFailedPutRecordsOperation(r *kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error) {
	return &kinesis.PutRecordsOutput{
			FailedRecordCount: aws.Int32(int32(len(r.Records))),
		},
		&types.ResourceNotFoundException{Message: aws.String("testing incorrect kinesis configuration")}
}

func TransientPutRecordsOperation(recoverAfter int) func(_ *kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error) {
	attempt := 0
	return func(r *kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error) {
		if attempt < recoverAfter {
			attempt++
			return &kinesis.PutRecordsOutput{
					FailedRecordCount: aws.Int32(int32(len(r.Records))),
				},
				&types.ProvisionedThroughputExceededException{Message: aws.String("testing throttled kinesis operation")}
		}
		return SuccessfulPutRecordsOperation(r)
	}
}

func TestBatchedExporter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		PutRecordsOP func(*kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error)
		shouldErr    bool
		isPermanent  bool
	}{
		{name: "Successful put to kinesis", PutRecordsOP: SuccessfulPutRecordsOperation, shouldErr: false, isPermanent: false},
		{name: "Invalid kinesis configuration", PutRecordsOP: HardFailedPutRecordsOperation, shouldErr: true, isPermanent: true},
		{name: "Test throttled kinesis operation", PutRecordsOP: TransientPutRecordsOperation(2), shouldErr: true, isPermanent: false},
	}

	bt := batch.New()
	for range 500 {
		assert.NoError(t, bt.AddRecord([]byte("foobar"), "fixed-key"))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			be, err := producer.NewBatcher(
				SetPutRecordsOperation(tc.PutRecordsOP),
				tc.name,
				producer.WithLogger(zaptest.NewLogger(t)),
			)
			require.NoError(t, err, "Must not error when creating BatchedExporter")
			require.NotNil(t, be, "Must have a valid client to use")

			err = be.Put(t.Context(), bt)
			if !tc.shouldErr {
				assert.NoError(t, err, "Must not have returned an error for this test case")
				return
			}

			assert.Error(t, err, "Must have returned an error for this test case")
			if tc.isPermanent {
				assert.True(t, consumererror.IsPermanent(err), "Must have returned a permanent error")
			}
		})
	}
}

func TestBatcherReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		descOp    func(*kinesis.DescribeStreamInput) (*kinesis.DescribeStreamOutput, error)
		shouldErr bool
	}{
		{
			name: "Successful DescribeStream",
			descOp: func(_ *kinesis.DescribeStreamInput) (*kinesis.DescribeStreamOutput, error) {
				return &kinesis.DescribeStreamOutput{}, nil
			},
			shouldErr: false,
		},
		{
			name: "LimitExceededException rate limit should log warning and not error",
			descOp: func(_ *kinesis.DescribeStreamInput) (*kinesis.DescribeStreamOutput, error) {
				return nil, &types.LimitExceededException{Message: aws.String("Rate exceeded for stream")}
			},
			shouldErr: false,
		},
		{
			name: "ResourceNotFoundException should return error",
			descOp: func(_ *kinesis.DescribeStreamInput) (*kinesis.DescribeStreamOutput, error) {
				return nil, &types.ResourceNotFoundException{Message: aws.String("Stream not found")}
			},
			shouldErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			be, err := producer.NewBatcher(
				&MockKinesisAPI{descOp: tc.descOp},
				"test-stream",
				producer.WithLogger(zaptest.NewLogger(t)),
			)
			require.NoError(t, err)
			err = be.Ready(t.Context())
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
