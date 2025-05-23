// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var testTime = time.Date(2021, 0o2, 0o1, 17, 32, 0o0, 0o0, time.UTC)

func Test_getTimeKeyPartitionHour(t *testing.T) {
	result := getTimeKeyPartitionHour(testTime)
	require.Equal(t, "year=2021/month=02/day=01/hour=17", result)
}

func Test_getTimeKeyPartitionMinute(t *testing.T) {
	result := getTimeKeyPartitionMinute(testTime)
	require.Equal(t, "year=2021/month=02/day=01/hour=17/minute=32", result)
}

func Test_s3Reader_getObjectPrefixForTime(t *testing.T) {
	type args struct {
		s3Prefix      string
		s3Partition   string
		filePrefix    string
		telemetryType string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "hour, prefix and file prefix",
			args: args{
				s3Prefix:      "prefix",
				s3Partition:   "hour",
				filePrefix:    "file",
				telemetryType: "traces",
			},
			want: "prefix/year=2021/month=02/day=01/hour=17/filetraces_",
		},
		{
			name: "minute, prefix and file prefix",
			args: args{
				s3Prefix:      "prefix",
				s3Partition:   "minute",
				filePrefix:    "file",
				telemetryType: "metrics",
			},
			want: "prefix/year=2021/month=02/day=01/hour=17/minute=32/filemetrics_",
		},
		{
			name: "hour, prefix and no file prefix",
			args: args{
				s3Prefix:      "prefix",
				s3Partition:   "hour",
				filePrefix:    "",
				telemetryType: "logs",
			},
			want: "prefix/year=2021/month=02/day=01/hour=17/logs_",
		},
		{
			name: "minute, no prefix and no file prefix",
			args: args{
				s3Prefix:      "",
				s3Partition:   "minute",
				filePrefix:    "",
				telemetryType: "metrics",
			},
			want: "year=2021/month=02/day=01/hour=17/minute=32/metrics_",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := s3TimeBasedReader{
				logger:      zap.NewNop(),
				s3Prefix:    test.args.s3Prefix,
				s3Partition: test.args.s3Partition,
				filePrefix:  test.args.filePrefix,
			}
			result := reader.getObjectPrefixForTime(testTime, test.args.telemetryType)
			require.Equal(t, test.want, result)
		})
	}
}

type mockGetObjectAPI func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)

func (m mockGetObjectAPI) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return m(ctx, params, optFns...)
}

type mockListObjectsAPI func(params *s3.ListObjectsV2Input) ListObjectsV2Pager

func (m mockListObjectsAPI) NewListObjectsV2Paginator(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
	return m(params)
}

type mockListObjectsV2Pager struct {
	PageNum int
	Pages   []*s3.ListObjectsV2Output
	Error   error
}

func (m *mockListObjectsV2Pager) HasMorePages() bool {
	return m.PageNum < len(m.Pages)
}

func (m *mockListObjectsV2Pager) NextPage(_ context.Context, _ ...func(*s3.Options)) (output *s3.ListObjectsV2Output, err error) {
	if m.Error != nil {
		return nil, m.Error
	}

	if m.PageNum >= len(m.Pages) {
		return nil, errors.New("no more pages")
	}
	output = m.Pages[m.PageNum]
	m.PageNum++
	return output, nil
}

func Test_readTelemetryForTime(t *testing.T) {
	testKey1 := "year=2021/month=02/day=01/hour=17/minute=32/traces_1"
	testKey2 := "year=2021/month=02/day=01/hour=17/minute=32/traces_2"
	reader := s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, "year=2021/month=02/day=01/hour=17/minute=32/traces_", *params.Prefix)

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{
								Key: &testKey1,
							},
						},
					},
					{
						Contents: []types.Object{
							{
								Key: &testKey2,
							},
						},
					},
				},
			}
		}),
		getObjectClient: mockGetObjectAPI(func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Contains(t, []string{testKey1, testKey2}, *params.Key)
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
			}, nil
		}),
		logger:      zap.NewNop(),
		s3Bucket:    "bucket",
		s3Partition: "minute",
		s3Prefix:    "",
		filePrefix:  "",
		startTime:   testTime,
		endTime:     testTime.Add(time.Minute),
	}

	dataCallbackKeys := make([]string, 0)

	err := reader.readTelemetryForTime(context.Background(), testTime, "traces", func(_ context.Context, key string, data []byte) error {
		t.Helper()
		require.Equal(t, "this is the body of the object", string(data))
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.Contains(t, dataCallbackKeys, testKey1)
	require.Contains(t, dataCallbackKeys, testKey2)
	require.NoError(t, err)
}

func Test_readTelemetryForTime_GetObjectError(t *testing.T) {
	testKey := "year=2021/month=02/day=01/hour=17/minute=32/traces_1"
	testError := errors.New("test error")
	reader := s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, "year=2021/month=02/day=01/hour=17/minute=32/traces_", *params.Prefix)

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{
								Key: &testKey,
							},
						},
					},
				},
			}
		}),
		getObjectClient: mockGetObjectAPI(func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, testKey, *params.Key)
			return nil, testError
		}),
		logger:      zap.NewNop(),
		s3Bucket:    "bucket",
		s3Partition: "minute",
		s3Prefix:    "",
		filePrefix:  "",
		startTime:   testTime,
		endTime:     testTime.Add(time.Minute),
	}

	err := reader.readTelemetryForTime(context.Background(), testTime, "traces", func(_ context.Context, _ string, _ []byte) error {
		t.Helper()
		t.Fail()
		return nil
	})
	require.Error(t, err, "test error")
}

func Test_readTelemetryForTime_ListObjectsNoResults(t *testing.T) {
	testKey := "year=2021/month=02/day=01/hour=17/minute=32/traces_1"
	reader := s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, "year=2021/month=02/day=01/hour=17/minute=32/traces_", *params.Prefix)

			return &mockListObjectsV2Pager{}
		}),
		getObjectClient: mockGetObjectAPI(func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, testKey, *params.Key)
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
			}, nil
		}),
		logger:      zap.NewNop(),
		s3Bucket:    "bucket",
		s3Partition: "minute",
		s3Prefix:    "",
		filePrefix:  "",
		startTime:   testTime,
		endTime:     testTime.Add(time.Minute),
	}

	err := reader.readTelemetryForTime(context.Background(), testTime, "traces", func(_ context.Context, _ string, _ []byte) error {
		t.Helper()
		t.Fail()
		return nil
	})
	require.NoError(t, err)
}

func Test_readTelemetryForTime_NextPageError(t *testing.T) {
	testKey := "year=2021/month=02/day=01/hour=17/minute=32/traces_1"
	testError := errors.New("test page error")
	reader := s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, "year=2021/month=02/day=01/hour=17/minute=32/traces_", *params.Prefix)

			return &mockListObjectsV2Pager{
				Error: testError,
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{
								Key: &testKey,
							},
						},
					},
				},
			}
		}),
		getObjectClient: mockGetObjectAPI(func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, testKey, *params.Key)
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
			}, nil
		}),
		logger:      zap.NewNop(),
		s3Bucket:    "bucket",
		s3Partition: "minute",
		s3Prefix:    "",
		filePrefix:  "",
		startTime:   testTime,
		endTime:     testTime.Add(time.Minute),
	}

	err := reader.readTelemetryForTime(context.Background(), testTime, "traces", func(_ context.Context, _ string, _ []byte) error {
		t.Helper()
		t.Fail()
		return nil
	})
	require.Error(t, err)
}

type mockNotifier struct {
	messages []statusNotification
}

func (m *mockNotifier) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (m *mockNotifier) Shutdown(_ context.Context) error {
	return nil
}

func (m *mockNotifier) SendStatus(_ context.Context, notification statusNotification) {
	m.messages = append(m.messages, notification)
}

func Test_readAll(t *testing.T) {
	reader := s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			key := fmt.Sprintf("%s%s", *params.Prefix, "1")
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{
								Key: &key,
							},
						},
					},
				},
			}
		}),
		getObjectClient: mockGetObjectAPI(func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
			}, nil
		}),
		logger:      zap.NewNop(),
		s3Bucket:    "bucket",
		s3Prefix:    "",
		s3Partition: "minute",
		filePrefix:  "",
		startTime:   testTime,
		endTime:     testTime.Add(time.Minute * 2),
	}

	dataCallbackKeys := make([]string, 0)

	err := reader.readAll(context.Background(), "traces", func(_ context.Context, key string, data []byte) error {
		t.Helper()
		require.Equal(t, "this is the body of the object", string(data))
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.NoError(t, err)
	require.Contains(t, dataCallbackKeys, "year=2021/month=02/day=01/hour=17/minute=32/traces_1")
	require.Contains(t, dataCallbackKeys, "year=2021/month=02/day=01/hour=17/minute=33/traces_1")
}

func Test_readAll_StatusMessages(t *testing.T) {
	notifier := mockNotifier{}
	reader := s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			key := fmt.Sprintf("%s%s", *params.Prefix, "1")
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{
								Key: &key,
							},
						},
					},
				},
			}
		}),
		getObjectClient: mockGetObjectAPI(func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
			}, nil
		}),
		logger:      zap.NewNop(),
		s3Bucket:    "bucket",
		s3Prefix:    "",
		s3Partition: "minute",
		filePrefix:  "",
		startTime:   testTime,
		endTime:     testTime.Add(time.Minute * 2),
		notifier:    &notifier,
	}

	dataCallbackKeys := make([]string, 0)

	err := reader.readAll(context.Background(), "traces", func(_ context.Context, key string, data []byte) error {
		t.Helper()
		require.Equal(t, "this is the body of the object", string(data))
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.NoError(t, err)
	require.Contains(t, dataCallbackKeys, "year=2021/month=02/day=01/hour=17/minute=32/traces_1")
	require.Contains(t, dataCallbackKeys, "year=2021/month=02/day=01/hour=17/minute=33/traces_1")
	require.Equal(t, []statusNotification{
		{
			TelemetryType: "traces",
			IngestStatus:  IngestStatusIngesting,
			StartTime:     testTime,
			EndTime:       testTime.Add(time.Minute * 2),
			IngestTime:    testTime,
		}, {
			TelemetryType: "traces",
			IngestStatus:  IngestStatusIngesting,
			StartTime:     testTime,
			EndTime:       testTime.Add(time.Minute * 2),
			IngestTime:    testTime.Add(time.Minute),
		}, {
			TelemetryType: "traces",
			IngestStatus:  IngestStatusCompleted,
			StartTime:     testTime,
			EndTime:       testTime.Add(time.Minute * 2),
			IngestTime:    testTime.Add(time.Minute * 2),
		},
	}, notifier.messages)
}

func Test_readAll_ContextDone(t *testing.T) {
	notifier := mockNotifier{}
	reader := s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			key := fmt.Sprintf("%s%s", *params.Prefix, "1")
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{
								Key: &key,
							},
						},
					},
				},
			}
		}),
		getObjectClient: mockGetObjectAPI(func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			t.Helper()
			require.Equal(t, "bucket", *params.Bucket)
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
			}, nil
		}),
		logger:      zap.NewNop(),
		s3Bucket:    "bucket",
		s3Prefix:    "",
		s3Partition: "minute",
		filePrefix:  "",
		startTime:   testTime,
		endTime:     testTime.Add(time.Minute * 2),
		notifier:    &notifier,
	}

	dataCallbackKeys := make([]string, 0)
	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()
	err := reader.readAll(ctx, "traces", func(_ context.Context, key string, _ []byte) error {
		t.Helper()
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.Error(t, err)
	require.Empty(t, dataCallbackKeys)
	require.Equal(t, []statusNotification{
		{
			TelemetryType: "traces",
			IngestStatus:  IngestStatusIngesting,
			StartTime:     testTime,
			EndTime:       testTime.Add(time.Minute * 2),
			IngestTime:    testTime,
		}, {
			TelemetryType:  "traces",
			IngestStatus:   IngestStatusFailed,
			StartTime:      testTime,
			EndTime:        testTime.Add(time.Minute * 2),
			IngestTime:     testTime,
			FailureMessage: "context canceled",
		},
	}, notifier.messages)
}
