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

	op func(*kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error)
}

func (mka *MockKinesisAPI) PutRecords(ctx context.Context, r *kinesis.PutRecordsInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordsOutput, error) {
	return mka.op(r)
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

func TransiantPutRecordsOperation(recoverAfter int) func(_ *kinesis.PutRecordsInput) (*kinesis.PutRecordsOutput, error) {
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
		{name: "Test throttled kinesis operation", PutRecordsOP: TransiantPutRecordsOperation(2), shouldErr: true, isPermanent: false},
	}

	bt := batch.New()
	for i := 0; i < 500; i++ {
		assert.NoError(t, bt.AddRecord([]byte("foobar"), "fixed-key"))
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			be, err := producer.NewBatcher(
				SetPutRecordsOperation(tc.PutRecordsOP),
				tc.name,
				producer.WithLogger(zaptest.NewLogger(t)),
			)
			require.NoError(t, err, "Must not error when creating BatchedExporter")
			require.NotNil(t, be, "Must have a valid client to use")

			err = be.Put(context.Background(), bt)
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
