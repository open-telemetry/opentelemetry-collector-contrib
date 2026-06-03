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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var testTime = time.Date(2021, 2, 1, 17, 32, 0, 0, time.UTC)

func Test_getTimeKey(t *testing.T) {
	result := getTimeKey(s3PartitionFormatDefault, testTime, time.UTC)
	require.Equal(t, "year=2021/month=02/day=01/hour=17/minute=32", result)
}

func Test_getTimeKey_WithTimezone(t *testing.T) {
	loc := time.FixedZone("JST", 9*60*60)
	result := getTimeKey(s3PartitionFormatDefault, testTime, loc)
	require.Equal(t, "year=2021/month=02/day=02/hour=02/minute=32", result)
}

func Test_s3Reader_getObjectPrefixForTime(t *testing.T) {
	type args struct {
		s3Prefix                   string
		s3PartitionFormat          string
		location                   *time.Location
		filePrefix                 string
		includeTelemetryTypeSuffix bool
		telemetryType              string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "hour, prefix and file prefix",
			args: args{
				s3Prefix:                   "prefix",
				s3PartitionFormat:          "year=%Y/month=%m/day=%d/hour=%H",
				location:                   time.UTC,
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "traces",
			},
			want: "prefix/year=2021/month=02/day=01/hour=17/filetraces_",
		},
		{
			name: "minute, prefix and file prefix",
			args: args{
				s3Prefix:                   "prefix",
				s3PartitionFormat:          s3PartitionFormatDefault,
				location:                   time.UTC,
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "metrics",
			},
			want: "prefix/year=2021/month=02/day=01/hour=17/minute=32/filemetrics_",
		},
		{
			name: "hour, prefix and no file prefix",
			args: args{
				s3Prefix:                   "prefix",
				s3PartitionFormat:          "year=%Y/month=%m/day=%d/hour=%H",
				location:                   time.UTC,
				filePrefix:                 "",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "logs",
			},
			want: "prefix/year=2021/month=02/day=01/hour=17/logs_",
		},
		{
			name: "minute, no prefix and no file prefix",
			args: args{
				s3Prefix:                   "",
				s3PartitionFormat:          s3PartitionFormatDefault,
				location:                   time.UTC,
				filePrefix:                 "",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "metrics",
			},
			want: "year=2021/month=02/day=01/hour=17/minute=32/metrics_",
		},
		{
			name: "prefix is / (should preserve leading slash)",
			args: args{
				s3Prefix:                   "/",
				s3PartitionFormat:          s3PartitionFormatDefault,
				location:                   time.UTC,
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "logs",
			},
			want: "/year=2021/month=02/day=01/hour=17/minute=32/filelogs_",
		},
		{
			name: "prefix is // (should preserve double leading slashes)",
			args: args{
				s3Prefix:                   "//",
				s3PartitionFormat:          "year=%Y/month=%m/day=%d/hour=%H",
				location:                   time.UTC,
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "metrics",
			},
			want: "//year=2021/month=02/day=01/hour=17/filemetrics_",
		},
		{
			name: "prefix starts and ends with slash /logs/",
			args: args{
				s3Prefix:                   "/logs/",
				s3PartitionFormat:          "year=%Y/month=%m/day=%d/hour=%H",
				location:                   time.UTC,
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "traces",
			},
			want: "/logs//year=2021/month=02/day=01/hour=17/filetraces_",
		},
		{
			name: "prefix starts and ends with double slash //raw//",
			args: args{
				s3Prefix:                   "//raw//",
				s3PartitionFormat:          s3PartitionFormatDefault,
				location:                   time.UTC,
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "logs",
			},
			want: "//raw///year=2021/month=02/day=01/hour=17/minute=32/filelogs_",
		},
		{
			name: "no telemetry type suffix",
			args: args{
				s3Prefix:                   "",
				s3PartitionFormat:          s3PartitionFormatDefault,
				location:                   time.UTC,
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: false,
				telemetryType:              "metrics",
			},
			want: "year=2021/month=02/day=01/hour=17/minute=32/file",
		},
		{
			name: "custom timezone applied",
			args: args{
				s3Prefix:                   "prefix",
				s3PartitionFormat:          s3PartitionFormatDefault,
				location:                   time.FixedZone("JST", 9*60*60),
				filePrefix:                 "file",
				includeTelemetryTypeSuffix: true,
				telemetryType:              "logs",
			},
			want: "prefix/year=2021/month=02/day=02/hour=02/minute=32/filelogs_",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := s3TimeBasedReader{
				logger:                         zap.NewNop(),
				s3Prefix:                       test.args.s3Prefix,
				s3PartitionFormat:              test.args.s3PartitionFormat,
				S3PartitionTimeLocation:        test.args.location,
				filePrefix:                     test.args.filePrefix,
				filePrefixIncludeTelemetryType: test.args.includeTelemetryTypeSuffix,
			}
			result := reader.getObjectPrefixForTime(testTime, test.args.telemetryType)
			require.Equal(t, test.want, result)
		})
	}
}

type mockSingleObjectAPI struct {
	getObjectFunc        func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	getObjectTaggingFunc func(ctx context.Context, params *s3.GetObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error)
	putObjectTaggingFunc func(ctx context.Context, params *s3.PutObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error)
}

func (m *mockSingleObjectAPI) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFunc != nil {
		return m.getObjectFunc(ctx, params, optFns...)
	}
	return nil, errors.New("GetObject not mocked")
}

func (m *mockSingleObjectAPI) GetObjectTagging(ctx context.Context, params *s3.GetObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
	if m.getObjectTaggingFunc != nil {
		return m.getObjectTaggingFunc(ctx, params, optFns...)
	}
	// Default to empty tag set if no mock function provided
	return &s3.GetObjectTaggingOutput{TagSet: []types.Tag{}}, nil
}

func (m *mockSingleObjectAPI) PutObjectTagging(ctx context.Context, params *s3.PutObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
	if m.putObjectTaggingFunc != nil {
		return m.putObjectTaggingFunc(ctx, params, optFns...)
	}
	// Default to success if no mock function provided
	return &s3.PutObjectTaggingOutput{}, nil
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
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Contains(t, []string{testKey1, testKey2}, *params.Key)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		s3Prefix:                       "",
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute),
	}

	dataCallbackKeys := make([]string, 0)

	err := reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, key string, data []byte) error {
		t.Helper()
		require.Equal(t, "this is the body of the object", string(data))
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.Contains(t, dataCallbackKeys, testKey1)
	require.Contains(t, dataCallbackKeys, testKey2)
	require.NoError(t, err)
}

func Test_readTelemetryForTime_skipTaggedObjects(t *testing.T) {
	testKey1 := "year=2021/month=02/day=01/hour=17/minute=32/traces_1"
	testKey2 := "year=2021/month=02/day=01/hour=17/minute=32/traces_2"
	testKey3 := "year=2021/month=02/day=01/hour=17/minute=32/traces_3"
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
								// Not tagged
								Key: &testKey1,
							},
						},
					},
					{
						Contents: []types.Object{
							{
								// Already tagged by the receiver as ingested
								Key: &testKey2,
							},
						},
					},
					{
						Contents: []types.Object{
							{
								// Has a tag, but not one set by the receiver
								Key: &testKey3,
							},
						},
					},
				},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				// testKey2 should not be fetched because it has the ingested tag
				require.Contains(t, []string{testKey1, testKey3}, *params.Key)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Contains(t, []string{testKey1, testKey2, testKey3}, *params.Key)
				var tagSet []types.Tag
				switch *params.Key {
				case testKey2:
					tagSet = []types.Tag{{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)}}
				case testKey3:
					tagSet = []types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tagSet}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		s3Prefix:                       "",
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute),
		skipIngestingTaggedObjects:     true,
	}

	dataCallbackKeys := make([]string, 0)

	err := reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, key string, data []byte) error {
		t.Helper()
		require.Equal(t, "this is the body of the object", string(data))
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.Equal(t, []string{testKey1, testKey3}, dataCallbackKeys)
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
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return nil, testError
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		s3Prefix:                       "",
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute),
	}

	err := reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, _ string, _ []byte) error {
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
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		s3Prefix:                       "",
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute),
	}

	err := reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, _ string, _ []byte) error {
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
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		s3Prefix:                       "",
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute),
	}

	err := reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, _ string, _ []byte) error {
		t.Helper()
		t.Fail()
		return nil
	})
	require.Error(t, err)
}

type mockNotifier struct {
	messages []statusNotification
}

func (*mockNotifier) Start(context.Context, component.Host) error {
	return nil
}

func (*mockNotifier) Shutdown(context.Context) error {
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
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3Prefix:                       "",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute * 2),
	}

	dataCallbackKeys := make([]string, 0)

	err := reader.readAll(t.Context(), "traces", func(_ context.Context, key string, data []byte) error {
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
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3Prefix:                       "",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute * 2),
		notifier:                       &notifier,
	}

	dataCallbackKeys := make([]string, 0)

	err := reader.readAll(t.Context(), "traces", func(_ context.Context, key string, data []byte) error {
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
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3Prefix:                       "",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		startTime:                      testTime,
		endTime:                        testTime.Add(time.Minute * 2),
		notifier:                       &notifier,
	}

	dataCallbackKeys := make([]string, 0)
	ctx, cancelFunc := context.WithCancel(t.Context())
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

func Test_readTelemetryForTime_WithTag(t *testing.T) {
	testKey := "year=2023/month=01/day=02/hour=03/minute=04/traces_test"
	taggedKeys := make([]string, 0)

	reader := &s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, "year=2023/month=01/day=02/hour=03/minute=04/traces_", *params.Prefix)

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{Key: &testKey},
						},
					},
				},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				taggedKeys = append(taggedKeys, *params.Key)
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		tagObjectAfterIngestion:        true, // Enable tagging
	}

	testTime, err := time.Parse(time.RFC3339, "2023-01-02T03:04:05Z")
	require.NoError(t, err)

	dataCallbackKeys := make([]string, 0)
	err = reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, key string, _ []byte) error {
		t.Helper()
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{testKey}, dataCallbackKeys)
	require.Equal(t, []string{testKey}, taggedKeys, "Object should be tagged after successful ingestion")
}

func Test_readTelemetryForTime_TagFailure(t *testing.T) {
	testKey := "year=2023/month=01/day=02/hour=03/minute=04/traces_test"

	reader := &s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, "year=2023/month=01/day=02/hour=03/minute=04/traces_", *params.Prefix)

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{Key: &testKey},
						},
					},
				},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return nil, errors.New("tagging failed")
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		tagObjectAfterIngestion:        true, // Enable tagging
	}

	testTime, err := time.Parse(time.RFC3339, "2023-01-02T03:04:05Z")
	require.NoError(t, err)

	dataCallbackKeys := make([]string, 0)
	// Should not return error even if tagging fails
	err = reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, key string, _ []byte) error {
		t.Helper()
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})
	require.NoError(t, err, "Should not fail when tagging fails")
	require.Equal(t, []string{testKey}, dataCallbackKeys, "Data should still be processed")
}

func Test_readTelemetryForTime_GetTagError(t *testing.T) {
	testKey := "year=2023/month=01/day=02/hour=03/minute=04/traces_test"

	reader := &s3TimeBasedReader{
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			require.Equal(t, "bucket", *params.Bucket)
			require.Equal(t, "year=2023/month=01/day=02/hour=03/minute=04/traces_", *params.Prefix)

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{
						Contents: []types.Object{
							{Key: &testKey},
						},
					},
				},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, params *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("this is the body of the object"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				t.Helper()
				require.Equal(t, "bucket", *params.Bucket)
				require.Equal(t, testKey, *params.Key)
				return nil, errors.New("failed to get object tags")
			},
		},
		logger:                         zap.NewNop(),
		s3Bucket:                       "bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		S3PartitionTimeLocation:        time.UTC,
		filePrefix:                     "",
		filePrefixIncludeTelemetryType: true,
		tagObjectAfterIngestion:        true,
		skipIngestingTaggedObjects:     true,
	}

	testTime, err := time.Parse(time.RFC3339, "2023-01-02T03:04:05Z")
	require.NoError(t, err)

	dataCallbackKeys := make([]string, 0)
	err = reader.readTelemetryForTime(t.Context(), testTime, "traces", func(_ context.Context, key string, _ []byte) error {
		t.Helper()
		dataCallbackKeys = append(dataCallbackKeys, key)
		return nil
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get object tags")
	require.Empty(t, dataCallbackKeys, "No data should be processed when GetObjectTagging fails")
}

func Test_determineTimestep(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		expectedStep   time.Duration
		expectedErrMsg string
	}{
		{
			name:         "minute partition",
			format:       s3PartitionFormatDefault,
			expectedStep: time.Minute,
		},
		{
			name:         "hour partition",
			format:       "year=%Y/month=%m/day=%d/hour=%H",
			expectedStep: time.Hour,
		},
		{
			name:           "no change detected",
			format:         "year=%Y/month=%m/day=%d",
			expectedErrMsg: "no time step found for partition format year=%Y/month=%m/day=%d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := determineTimestep(tt.format)
			if tt.expectedErrMsg != "" {
				require.EqualError(t, err, tt.expectedErrMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedStep, step)
		})
	}
}
