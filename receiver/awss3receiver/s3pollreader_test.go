// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func Test_truncateToPartitionBoundary(t *testing.T) {
	locUTC := time.UTC
	locJST := time.FixedZone("JST", 9*60*60)
	locIST := time.FixedZone("IST", int(5.5*3600)) // +05:30

	testTime := time.Date(2026, 11, 6, 14, 23, 45, 123456789, time.UTC)

	t.Run("minute truncation UTC", func(t *testing.T) {
		res := truncateToPartitionBoundary(testTime, time.Minute, locUTC)
		expected := time.Date(2026, 11, 6, 14, 23, 0, 0, locUTC)
		require.Equal(t, expected, res)
	})

	t.Run("hour truncation UTC", func(t *testing.T) {
		res := truncateToPartitionBoundary(testTime, time.Hour, locUTC)
		expected := time.Date(2026, 11, 6, 14, 0, 0, 0, locUTC)
		require.Equal(t, expected, res)
	})

	t.Run("day truncation UTC", func(t *testing.T) {
		res := truncateToPartitionBoundary(testTime, 24*time.Hour, locUTC)
		expected := time.Date(2026, 11, 6, 0, 0, 0, 0, locUTC)
		require.Equal(t, expected, res)
	})

	t.Run("hour truncation with non-zero minute timezone (IST)", func(t *testing.T) {
		// testTime in IST is 19:53:45 IST
		res := truncateToPartitionBoundary(testTime, time.Hour, locIST)
		expected := time.Date(2026, 11, 6, 19, 0, 0, 0, locIST)
		require.Equal(t, expected, res)
		require.Equal(t, 19, res.Hour())
		require.Equal(t, 0, res.Minute())
	})

	t.Run("day truncation JST", func(t *testing.T) {
		// testTime in JST is 23:23:45 JST
		res := truncateToPartitionBoundary(testTime, 24*time.Hour, locJST)
		expected := time.Date(2026, 11, 6, 0, 0, 0, 0, locJST)
		require.Equal(t, expected, res)
	})
}

func Test_newS3PollingReader(t *testing.T) {
	ctx := t.Context()

	t.Run("valid configuration", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "mybucket",
				S3PartitionFormat:          s3PartitionFormatDefault,
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			StartTime: "2026-11-06 10:00",
			Poll: &PollConfig{
				Interval: 5 * time.Minute,
				Lookback: 15 * time.Minute,
			},
		}
		reader, err := newS3PollingReader(ctx, zap.NewNop(), cfg)
		require.NoError(t, err)
		require.NotNil(t, reader)
		assert.Equal(t, 5*time.Minute, reader.pollInterval)
		assert.Equal(t, 15*time.Minute, reader.lookback)
		require.NotNil(t, reader.startTime)
		assert.Equal(t, time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC), reader.startTime.UTC())
	})

	t.Run("default lookback to interval", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "mybucket",
				S3PartitionFormat:          s3PartitionFormatDefault,
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			Poll: &PollConfig{
				Interval: 2 * time.Minute,
				Lookback: 0,
			},
		}
		reader, err := newS3PollingReader(ctx, zap.NewNop(), cfg)
		require.NoError(t, err)
		require.NotNil(t, reader)
		assert.Equal(t, 2*time.Minute, reader.lookback)
		assert.Nil(t, reader.startTime)
	})

	t.Run("invalid start time", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "mybucket",
				S3PartitionFormat:          s3PartitionFormatDefault,
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			StartTime: "not-a-valid-time",
			Poll: &PollConfig{
				Interval: 5 * time.Minute,
			},
		}
		_, err := newS3PollingReader(ctx, zap.NewNop(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to parse starttime")
	})

	t.Run("invalid partition format", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "mybucket",
				S3PartitionFormat:          "year=%Y/month=%m", // no timestep detected
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			Poll: &PollConfig{
				Interval: 5 * time.Minute,
			},
		}
		_, err := newS3PollingReader(ctx, zap.NewNop(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no time step found for partition format")
	})

	t.Run("invalid timezone", func(t *testing.T) {
		cfg := &Config{
			S3Downloader: S3DownloaderConfig{
				S3Bucket:                   "mybucket",
				S3PartitionFormat:          s3PartitionFormatDefault,
				S3PartitionTimezone:        "Invalid/Timezone",
				TagObjectAfterIngestion:    true,
				SkipIngestingTaggedObjects: true,
			},
			Poll: &PollConfig{
				Interval: 5 * time.Minute,
			},
		}
		_, err := newS3PollingReader(ctx, zap.NewNop(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid S3 partition timezone")
	})
}

func Test_s3PollingReader_readAll_InitialPollAndRepeatedPolling(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 12, 5, 0, 0, time.UTC)
	var timeMu sync.Mutex
	getNow := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return simulatedTime
	}

	tickerChan := make(chan time.Time, 1)

	var (
		ingestMu      sync.Mutex
		ingestedKeys  []string
		listedPrefix  []string
		taggedObjects = make(map[string]bool)
	)

	key1 := "year=2026/month=11/day=06/hour=12/minute=04/traces_1.json"
	key2 := "year=2026/month=11/day=06/hour=12/minute=05/traces_2.json"
	key3 := "year=2026/month=11/day=06/hour=12/minute=06/traces_3.json"

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Minute,
		lookback:                       2 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        getNow,
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			ingestMu.Lock()
			prefix := *params.Prefix
			listedPrefix = append(listedPrefix, prefix)
			ingestMu.Unlock()

			var contents []types.Object
			switch prefix {
			case "year=2026/month=11/day=06/hour=12/minute=04/traces_":
				contents = []types.Object{{Key: &key1}}
			case "year=2026/month=11/day=06/hour=12/minute=05/traces_":
				contents = []types.Object{{Key: &key2}}
			case "year=2026/month=11/day=06/hour=12/minute=06/traces_":
				contents = []types.Object{{Key: &key3}}
			}

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{Contents: contents},
				},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("dummy content"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				ingestMu.Lock()
				defer ingestMu.Unlock()
				var tags []types.Tag
				if taggedObjects[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				ingestMu.Lock()
				taggedObjects[*params.Key] = true
				ingestMu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, content io.Reader) error {
			data, err := io.ReadAll(content)
			assert.NoError(t, err)
			assert.Equal(t, "dummy content", string(data))
			ingestMu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			ingestMu.Unlock()
			return nil
		})
	}()

	// Wait for initial pass covering minutes 03, 04, 05 (lookback = 2m from 12:05)
	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) >= 2
	}, 2*time.Second, 10*time.Millisecond)

	ingestMu.Lock()
	assert.Contains(t, ingestedKeys, key1)
	assert.Contains(t, ingestedKeys, key2)
	assert.NotContains(t, ingestedKeys, key3)
	ingestMu.Unlock()

	// Advance wall-clock time and trigger ticker
	timeMu.Lock()
	simulatedTime = simulatedTime.Add(1 * time.Minute) // 12:06
	timeMu.Unlock()

	tickerChan <- simulatedTime

	// Wait for second pass covering minutes 04, 05, 06
	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) >= 3
	}, 2*time.Second, 10*time.Millisecond)

	ingestMu.Lock()
	assert.Contains(t, ingestedKeys, key3)
	assert.Equal(t, []string{key1, key2, key3}, ingestedKeys)
	ingestMu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_HistoricalBackfill(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 10, 5, 0, 0, time.UTC)
	startTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	var (
		ingestMu     sync.Mutex
		ingestedKeys []string
	)

	key0 := "year=2026/month=11/day=06/hour=10/minute=00/traces_0.json"
	key1 := "year=2026/month=11/day=06/hour=10/minute=01/traces_1.json"
	key2 := "year=2026/month=11/day=06/hour=10/minute=02/traces_2.json"

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		startTime:                      &startTime,
		pollInterval:                   1 * time.Minute,
		lookback:                       2 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        func() time.Time { return simulatedTime },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return make(chan time.Time), func() {} // does not fire in this test
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			var contents []types.Object
			switch *params.Prefix {
			case "year=2026/month=11/day=06/hour=10/minute=00/traces_":
				contents = []types.Object{{Key: &key0}}
			case "year=2026/month=11/day=06/hour=10/minute=01/traces_":
				contents = []types.Object{{Key: &key1}}
			case "year=2026/month=11/day=06/hour=10/minute=02/traces_":
				contents = []types.Object{{Key: &key2}}
			}
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("backfill-payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, _ *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				return &s3.GetObjectTaggingOutput{TagSet: []types.Tag{}}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, _ *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			ingestMu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			ingestMu.Unlock()
			return nil
		})
	}()

	// Wait for backfill to ingest all 3 historical keys
	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) >= 3
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)

	ingestMu.Lock()
	assert.Contains(t, ingestedKeys, key0)
	assert.Contains(t, ingestedKeys, key1)
	assert.Contains(t, ingestedKeys, key2)
	ingestMu.Unlock()
}

func Test_s3PollingReader_readAll_HistoricalBackfill_TransientErrorRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 10, 2, 0, 0, time.UTC)
	startTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	tickerChan := make(chan time.Time, 1)

	var (
		ingestMu       sync.Mutex
		ingestedKeys   []string
		attemptPerPart = make(map[string]int)
	)

	key0 := "year=2026/month=11/day=06/hour=10/minute=00/traces_0.json"
	key1 := "year=2026/month=11/day=06/hour=10/minute=01/traces_1.json"
	key2 := "year=2026/month=11/day=06/hour=10/minute=02/traces_2.json"

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		startTime:                      &startTime,
		pollInterval:                   1 * time.Minute,
		lookback:                       1 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        func() time.Time { return simulatedTime },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			ingestMu.Lock()
			prefix := *params.Prefix
			attempt := attemptPerPart[prefix]
			attemptPerPart[prefix]++
			ingestMu.Unlock()

			// Fail minute 01 on the first attempt
			if prefix == "year=2026/month=11/day=06/hour=10/minute=01/traces_" && attempt == 0 {
				return &mockListObjectsV2Pager{
					Error: errors.New("transient S3 error"),
					Pages: []*s3.ListObjectsV2Output{{}},
				}
			}

			var contents []types.Object
			switch prefix {
			case "year=2026/month=11/day=06/hour=10/minute=00/traces_":
				contents = []types.Object{{Key: &key0}}
			case "year=2026/month=11/day=06/hour=10/minute=01/traces_":
				contents = []types.Object{{Key: &key1}}
			case "year=2026/month=11/day=06/hour=10/minute=02/traces_":
				contents = []types.Object{{Key: &key2}}
			}
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("backfill-payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, _ *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				return &s3.GetObjectTaggingOutput{TagSet: []types.Tag{}}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, _ *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			ingestMu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			ingestMu.Unlock()
			return nil
		})
	}()

	// Wait for initial pass to process minute 00 and fail on minute 01
	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) == 1 && ingestedKeys[0] == key0
	}, 2*time.Second, 10*time.Millisecond)

	// Trigger next poll cycle: backfill should resume from minute 01 (not restart from minute 00)
	tickerChan <- simulatedTime

	// Wait for minute 01 and 02 to be completed
	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) >= 3
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)

	ingestMu.Lock()
	assert.Equal(t, 1, attemptPerPart["year=2026/month=11/day=06/hour=10/minute=00/traces_"], "minute 00 was not re-polled after succeeding")
	assert.Equal(t, 2, attemptPerPart["year=2026/month=11/day=06/hour=10/minute=01/traces_"], "minute 01 was retried")
	assert.Contains(t, ingestedKeys, key1)
	assert.Contains(t, ingestedKeys, key2)
	ingestMu.Unlock()
}

func Test_s3PollingReader_readAll_SkipTaggedAndLateArrivingObjects(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 12, 5, 0, 0, time.UTC)
	tickerChan := make(chan time.Time, 1)

	var (
		ingestMu     sync.Mutex
		ingestedKeys []string
		taggedMap    = make(map[string]bool)
		tagCalls     = make(map[string]int)
	)

	keyRegular := "year=2026/month=11/day=06/hour=12/minute=05/traces_reg.json"
	keyLate := "year=2026/month=11/day=06/hour=12/minute=04/traces_late.json"

	var lateArrived atomic.Bool

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Minute,
		lookback:                       2 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        func() time.Time { return simulatedTime },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			var contents []types.Object
			switch *params.Prefix {
			case "year=2026/month=11/day=06/hour=12/minute=05/traces_":
				contents = []types.Object{{Key: &keyRegular}}
			case "year=2026/month=11/day=06/hour=12/minute=04/traces_":
				if lateArrived.Load() {
					contents = []types.Object{{Key: &keyLate}}
				}
			}
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				ingestMu.Lock()
				defer ingestMu.Unlock()
				var tags []types.Tag
				if taggedMap[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				ingestMu.Lock()
				taggedMap[*params.Key] = true
				tagCalls[*params.Key]++
				ingestMu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			ingestMu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			ingestMu.Unlock()
			return nil
		})
	}()

	// Wait for pass 1
	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) == 1 && ingestedKeys[0] == keyRegular
	}, 2*time.Second, 10*time.Millisecond)

	// Now late-arriving object lands in partition 12:04 (within 2-minute lookback)
	lateArrived.Store(true)

	// Trigger pass 2
	tickerChan <- simulatedTime

	// Wait for late-arriving object to be ingested
	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) == 2
	}, 2*time.Second, 10*time.Millisecond)

	ingestMu.Lock()
	assert.Contains(t, ingestedKeys, keyLate)
	assert.Equal(t, 1, tagCalls[keyRegular], "keyRegular was not re-tagged on pass 2")
	assert.Equal(t, 1, tagCalls[keyLate], "keyLate was tagged exactly once")
	ingestMu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_NoFuturePartitions(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 12, 5, 30, 0, time.UTC)
	var (
		mu           sync.Mutex
		scannedTimes []time.Time
	)

	reader := &s3PollingReader{
		logger:                  zap.NewNop(),
		s3Bucket:                "test-bucket",
		s3PartitionFormat:       s3PartitionFormatDefault,
		s3PartitionTimeLocation: time.UTC,
		pollInterval:            1 * time.Minute,
		lookback:                1 * time.Minute,
		timeStep:                time.Minute,
		nowFunc:                 func() time.Time { return simulatedTime },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return make(chan time.Time), func() {}
		},
		listObjectsClient: mockListObjectsAPI(func(_ *s3.ListObjectsV2Input) ListObjectsV2Pager {
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{}}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.scanRange(ctx, simulatedTime.Add(-1*time.Minute), simulatedTime, "traces", func(_ context.Context, _ string, _ io.Reader) error {
			return nil
		}, func(t time.Time) {
			mu.Lock()
			scannedTimes = append(scannedTimes, t)
			mu.Unlock()
		})
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	mu.Lock()
	for _, st := range scannedTimes {
		assert.False(t, st.After(simulatedTime), "scanned partition must never be in the future")
	}
	mu.Unlock()
}

func Test_s3PollingReader_readAll_Pagination(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	key1 := "page1_obj"
	key2 := "page2_obj"
	var (
		ingestMu     sync.Mutex
		ingestedKeys []string
	)

	reader := &s3PollingReader{
		logger:                  zap.NewNop(),
		s3Bucket:                "test-bucket",
		s3PartitionFormat:       s3PartitionFormatDefault,
		s3PartitionTimeLocation: time.UTC,
		pollInterval:            1 * time.Minute,
		lookback:                1 * time.Minute,
		timeStep:                time.Minute,
		nowFunc:                 func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC) },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return make(chan time.Time), func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(_ *s3.ListObjectsV2Input) ListObjectsV2Pager {
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{Contents: []types.Object{{Key: &key1}}},
					{Contents: []types.Object{{Key: &key2}}},
				},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, _ *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				return &s3.GetObjectTaggingOutput{TagSet: []types.Tag{}}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, _ *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			ingestMu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			ingestMu.Unlock()
			return nil
		})
	}()

	require.Eventually(t, func() bool {
		ingestMu.Lock()
		defer ingestMu.Unlock()
		return len(ingestedKeys) >= 2
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)

	ingestMu.Lock()
	defer ingestMu.Unlock()
	assert.Contains(t, ingestedKeys, key1)
	assert.Contains(t, ingestedKeys, key2)
}

func Test_s3PollingReader_readAll_TagFailure_DoesNotCrash(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	key := "fail_tag_obj"
	var callbackCalled bool
	var mu sync.Mutex

	reader := &s3PollingReader{
		logger:                  zap.NewNop(),
		s3Bucket:                "test-bucket",
		s3PartitionFormat:       s3PartitionFormatDefault,
		s3PartitionTimeLocation: time.UTC,
		pollInterval:            1 * time.Minute,
		lookback:                1 * time.Minute,
		timeStep:                time.Minute,
		nowFunc:                 func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC) },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return make(chan time.Time), func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(_ *s3.ListObjectsV2Input) ListObjectsV2Pager {
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{
					{Contents: []types.Object{{Key: &key}}},
				},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, _ *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				return &s3.GetObjectTaggingOutput{TagSet: []types.Tag{}}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, _ *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				return nil, errors.New("S3 put tagging internal error")
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, _ string, _ io.Reader) error {
			mu.Lock()
			callbackCalled = true
			mu.Unlock()
			return nil
		})
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return callbackCalled
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_NoOverlappingPasses(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	tickerChan := make(chan time.Time, 10)
	var activePasses atomic.Int32
	var maxConcurrentPasses atomic.Int32

	reader := &s3PollingReader{
		logger:                  zap.NewNop(),
		s3Bucket:                "test-bucket",
		s3PartitionFormat:       s3PartitionFormatDefault,
		s3PartitionTimeLocation: time.UTC,
		pollInterval:            1 * time.Minute,
		lookback:                1 * time.Minute,
		timeStep:                time.Minute,
		nowFunc:                 func() time.Time { return time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC) },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		listObjectsClient: mockListObjectsAPI(func(_ *s3.ListObjectsV2Input) ListObjectsV2Pager {
			current := activePasses.Add(1)
			for {
				currMax := maxConcurrentPasses.Load()
				if current <= currMax || maxConcurrentPasses.CompareAndSwap(currMax, current) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			activePasses.Add(-1)
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{}}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, _ string, _ io.Reader) error {
			return nil
		})
	}()

	// Send multiple ticks quickly
	for range 5 {
		tickerChan <- time.Now()
	}

	time.Sleep(150 * time.Millisecond)
	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)

	assert.Equal(t, int32(1), maxConcurrentPasses.Load(), "Passes must run strictly sequentially without overlapping")
}

func Test_advancePartitionTime(t *testing.T) {
	locUTC := time.UTC
	locNY, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	t.Run("minute advancement", func(t *testing.T) {
		t1 := time.Date(2026, 11, 6, 14, 23, 0, 0, locUTC)
		t2 := advancePartitionTime(t1, time.Minute, locUTC)
		assert.Equal(t, time.Date(2026, 11, 6, 14, 24, 0, 0, locUTC), t2)
	})

	t.Run("hour advancement", func(t *testing.T) {
		t1 := time.Date(2026, 11, 6, 14, 0, 0, 0, locUTC)
		t2 := advancePartitionTime(t1, time.Hour, locUTC)
		assert.Equal(t, time.Date(2026, 11, 6, 15, 0, 0, 0, locUTC), t2)
	})

	t.Run("day advancement standard UTC", func(t *testing.T) {
		t1 := time.Date(2026, 11, 6, 0, 0, 0, 0, locUTC)
		t2 := advancePartitionTime(t1, 24*time.Hour, locUTC)
		assert.Equal(t, time.Date(2026, 11, 7, 0, 0, 0, 0, locUTC), t2)
	})

	t.Run("day advancement across DST fall back (25 hours)", func(t *testing.T) {
		// November 1, 2026 in America/New_York has 25 hours due to DST fallback at 2 AM
		t1 := time.Date(2026, 11, 1, 0, 0, 0, 0, locNY)
		t2 := advancePartitionTime(t1, 24*time.Hour, locNY)
		assert.Equal(t, time.Date(2026, 11, 2, 0, 0, 0, 0, locNY), t2)
		assert.Equal(t, 0, t2.Hour())
		assert.Equal(t, 2, t2.Day())
	})

	t.Run("day advancement across DST spring forward (23 hours)", func(t *testing.T) {
		// March 8, 2026 in America/New_York has 23 hours due to DST spring forward at 2 AM
		t1 := time.Date(2026, 3, 8, 0, 0, 0, 0, locNY)
		t2 := advancePartitionTime(t1, 24*time.Hour, locNY)
		assert.Equal(t, time.Date(2026, 3, 9, 0, 0, 0, 0, locNY), t2)
		assert.Equal(t, 0, t2.Hour())
		assert.Equal(t, 9, t2.Day())
	})
}

func Test_scanRange_StartAfterEnd_ReturnsNil(t *testing.T) {
	ctx := t.Context()
	var callbackCalled bool

	reader := &s3PollingReader{
		logger:                  zap.NewNop(),
		s3Bucket:                "test-bucket",
		s3PartitionFormat:       s3PartitionFormatDefault,
		s3PartitionTimeLocation: time.UTC,
		timeStep:                time.Minute,
	}

	start := time.Date(2026, 11, 6, 14, 30, 0, 0, time.UTC)
	end := time.Date(2026, 11, 6, 14, 20, 0, 0, time.UTC)

	err := reader.scanRange(ctx, start, end, "traces", func(_ context.Context, _ string, _ io.Reader) error {
		callbackCalled = true
		return nil
	}, nil)

	require.NoError(t, err)
	assert.False(t, callbackCalled, "callback must not be invoked when start is after end")
}

func Test_scanRange_Daily_DST_Transition(t *testing.T) {
	ctx := t.Context()
	locNY, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	var (
		mu           sync.Mutex
		scannedTimes []time.Time
	)

	reader := &s3PollingReader{
		logger:                  zap.NewNop(),
		s3Bucket:                "test-bucket",
		s3PartitionFormat:       "year=%Y/month=%m/day=%d",
		s3PartitionTimeLocation: locNY,
		timeStep:                24 * time.Hour,
		listObjectsClient: mockListObjectsAPI(func(_ *s3.ListObjectsV2Input) ListObjectsV2Pager {
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{}}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{},
	}

	// Range covers Nov 1 (DST 25-hour day), Nov 2, and Nov 3
	start := time.Date(2026, 11, 1, 10, 0, 0, 0, locNY)
	end := time.Date(2026, 11, 3, 15, 0, 0, 0, locNY)

	err = reader.scanRange(ctx, start, end, "traces", func(_ context.Context, _ string, _ io.Reader) error {
		return nil
	}, func(t time.Time) {
		mu.Lock()
		scannedTimes = append(scannedTimes, t)
		mu.Unlock()
	})

	require.NoError(t, err)
	require.Len(t, scannedTimes, 3)
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, locNY), scannedTimes[0])
	assert.Equal(t, time.Date(2026, 11, 2, 0, 0, 0, 0, locNY), scannedTimes[1])
	assert.Equal(t, time.Date(2026, 11, 3, 0, 0, 0, 0, locNY), scannedTimes[2])
}

func Test_s3PollingReader_readAll_SlowPass_ExceedingIntervalAndLookback(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 12, 5, 0, 0, time.UTC)
	var timeMu sync.Mutex
	getNow := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return simulatedTime
	}

	tickerChan := make(chan time.Time, 1)

	var (
		mu            sync.Mutex
		ingestedKeys  []string
		taggedObjects = make(map[string]bool)
	)

	key04 := "year=2026/month=11/day=06/hour=12/minute=04/traces_04.json"
	key05 := "year=2026/month=11/day=06/hour=12/minute=05/traces_05.json"
	key06 := "year=2026/month=11/day=06/hour=12/minute=06/traces_06.json"
	// minute 07 is empty
	key08 := "year=2026/month=11/day=06/hour=12/minute=08/traces_08.json"

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Minute,
		lookback:                       1 * time.Minute, // lookback = interval
		timeStep:                       time.Minute,
		nowFunc:                        getNow,
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			var contents []types.Object
			switch *params.Prefix {
			case "year=2026/month=11/day=06/hour=12/minute=04/traces_":
				contents = []types.Object{{Key: &key04}}
			case "year=2026/month=11/day=06/hour=12/minute=05/traces_":
				contents = []types.Object{{Key: &key05}}
			case "year=2026/month=11/day=06/hour=12/minute=06/traces_":
				contents = []types.Object{{Key: &key06}}
			case "year=2026/month=11/day=06/hour=12/minute=07/traces_":
				// Empty partition
				contents = []types.Object{}
			case "year=2026/month=11/day=06/hour=12/minute=08/traces_":
				contents = []types.Object{{Key: &key08}}
			}

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				var tags []types.Tag
				if taggedObjects[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				mu.Lock()
				taggedObjects[*params.Key] = true
				mu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			mu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			mu.Unlock()
			return nil
		})
	}()

	// Wait for pass 1 covering minutes 04 and 05
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 2
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []string{key04, key05}, ingestedKeys)
	mu.Unlock()

	// Advance time by 3 minutes to 12:08 (longer than 1m lookback) and trigger pass 2
	timeMu.Lock()
	simulatedTime = simulatedTime.Add(3 * time.Minute) // 12:08
	timeMu.Unlock()

	tickerChan <- simulatedTime

	// Wait for pass 2 covering minutes 06, 07 (empty), and 08
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 4
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []string{key04, key05, key06, key08}, ingestedKeys, "Slow pass must not skip intermediate partitions 06 or 07")
	mu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_TransientFailure_MidwayRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 12, 5, 0, 0, time.UTC)
	var timeMu sync.Mutex
	getNow := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return simulatedTime
	}

	tickerChan := make(chan time.Time, 1)

	var (
		mu            sync.Mutex
		ingestedKeys  []string
		taggedObjects = make(map[string]bool)
		failMinute05  atomic.Bool
	)
	failMinute05.Store(true)

	key04 := "year=2026/month=11/day=06/hour=12/minute=04/traces_04.json"
	key05 := "year=2026/month=11/day=06/hour=12/minute=05/traces_05.json"
	key06 := "year=2026/month=11/day=06/hour=12/minute=06/traces_06.json"
	key08 := "year=2026/month=11/day=06/hour=12/minute=08/traces_08.json"

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Minute,
		lookback:                       1 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        getNow,
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			prefix := *params.Prefix
			if prefix == "year=2026/month=11/day=06/hour=12/minute=05/traces_" && failMinute05.Load() {
				return &mockListObjectsV2Pager{
					Error: errors.New("simulated transient S3 error on minute 05"),
					Pages: []*s3.ListObjectsV2Output{{}},
				}
			}

			var contents []types.Object
			switch prefix {
			case "year=2026/month=11/day=06/hour=12/minute=04/traces_":
				contents = []types.Object{{Key: &key04}}
			case "year=2026/month=11/day=06/hour=12/minute=05/traces_":
				contents = []types.Object{{Key: &key05}}
			case "year=2026/month=11/day=06/hour=12/minute=06/traces_":
				contents = []types.Object{{Key: &key06}}
			case "year=2026/month=11/day=06/hour=12/minute=08/traces_":
				contents = []types.Object{{Key: &key08}}
			}

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				var tags []types.Tag
				if taggedObjects[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				mu.Lock()
				taggedObjects[*params.Key] = true
				mu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			mu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			mu.Unlock()
			return nil
		})
	}()

	// Wait for pass 1 to ingest minute 04 and fail on minute 05
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) == 1 && ingestedKeys[0] == key04
	}, 2*time.Second, 10*time.Millisecond)

	// Clear error on minute 05
	failMinute05.Store(false)

	// Advance time to 12:08 (3 minutes later, well past the 1m lookback) and trigger pass 2
	timeMu.Lock()
	simulatedTime = simulatedTime.Add(3 * time.Minute) // 12:08
	timeMu.Unlock()

	tickerChan <- simulatedTime

	// Wait for pass 2 to retry 05 and complete through 08
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 4
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []string{key04, key05, key06, key08}, ingestedKeys, "Failed partition 05 must be retried and subsequent partitions scanned without gaps")
	mu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_ConsecutiveSlowPasses(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 12, 2, 0, 0, time.UTC)
	var timeMu sync.Mutex
	getNow := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return simulatedTime
	}

	tickerChan := make(chan time.Time, 1)

	var (
		mu            sync.Mutex
		ingestedKeys  []string
		taggedObjects = make(map[string]bool)
	)

	keys := map[string]string{
		"year=2026/month=11/day=06/hour=12/minute=01/traces_": "traces_01.json",
		"year=2026/month=11/day=06/hour=12/minute=02/traces_": "traces_02.json",
		"year=2026/month=11/day=06/hour=12/minute=03/traces_": "traces_03.json",
		"year=2026/month=11/day=06/hour=12/minute=04/traces_": "traces_04.json",
		"year=2026/month=11/day=06/hour=12/minute=05/traces_": "traces_05.json",
		"year=2026/month=11/day=06/hour=12/minute=06/traces_": "traces_06.json",
		"year=2026/month=11/day=06/hour=12/minute=07/traces_": "traces_07.json",
		"year=2026/month=11/day=06/hour=12/minute=08/traces_": "traces_08.json",
	}

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Minute,
		lookback:                       1 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        getNow,
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			var contents []types.Object
			if val, ok := keys[*params.Prefix]; ok {
				k := val
				contents = []types.Object{{Key: &k}}
			}
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				var tags []types.Tag
				if taggedObjects[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				mu.Lock()
				taggedObjects[*params.Key] = true
				mu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			mu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			mu.Unlock()
			return nil
		})
	}()

	// Pass 1: covering 01, 02
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 2
	}, 2*time.Second, 10*time.Millisecond)

	// Time jump 3 minutes to 12:05 (slow pass 1)
	timeMu.Lock()
	simulatedTime = simulatedTime.Add(3 * time.Minute)
	timeMu.Unlock()
	tickerChan <- simulatedTime

	// Pass 2: covering 03, 04, 05
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 5
	}, 2*time.Second, 10*time.Millisecond)

	// Time jump 3 minutes to 12:08 (slow pass 2)
	timeMu.Lock()
	simulatedTime = simulatedTime.Add(3 * time.Minute)
	timeMu.Unlock()
	tickerChan <- simulatedTime

	// Pass 3: covering 06, 07, 08
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 8
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	expected := []string{
		"traces_01.json", "traces_02.json",
		"traces_03.json", "traces_04.json", "traces_05.json",
		"traces_06.json", "traces_07.json", "traces_08.json",
	}
	assert.Equal(t, expected, ingestedKeys, "Consecutive slow passes must ingest all partitions sequentially without gaps")
	mu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_HistoricalBackfill_TransitionToRolling(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 10, 2, 0, 0, time.UTC)
	startTime := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	var timeMu sync.Mutex
	getNow := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return simulatedTime
	}

	tickerChan := make(chan time.Time, 1)

	var (
		mu            sync.Mutex
		ingestedKeys  []string
		taggedObjects = make(map[string]bool)
	)

	keys := map[string]string{
		"year=2026/month=11/day=06/hour=10/minute=00/traces_": "traces_00.json",
		"year=2026/month=11/day=06/hour=10/minute=01/traces_": "traces_01.json",
		"year=2026/month=11/day=06/hour=10/minute=02/traces_": "traces_02.json",
		"year=2026/month=11/day=06/hour=10/minute=03/traces_": "traces_03.json",
		"year=2026/month=11/day=06/hour=10/minute=04/traces_": "traces_04.json",
		"year=2026/month=11/day=06/hour=10/minute=05/traces_": "traces_05.json",
	}

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		startTime:                      &startTime,
		pollInterval:                   1 * time.Minute,
		lookback:                       1 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        getNow,
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			var contents []types.Object
			if val, ok := keys[*params.Prefix]; ok {
				k := val
				contents = []types.Object{{Key: &k}}
			}
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				var tags []types.Tag
				if taggedObjects[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				mu.Lock()
				taggedObjects[*params.Key] = true
				mu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			mu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			mu.Unlock()
			return nil
		})
	}()

	// Wait for backfill pass covering 10:00, 10:01, 10:02
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 3
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []string{"traces_00.json", "traces_01.json", "traces_02.json"}, ingestedKeys)
	mu.Unlock()

	// Advance time to 10:05 (3 minutes after backfill target) and trigger first rolling pass
	timeMu.Lock()
	simulatedTime = time.Date(2026, 11, 6, 10, 5, 0, 0, time.UTC)
	timeMu.Unlock()
	tickerChan <- simulatedTime

	// Wait for rolling pass: must pick up seamlessly from 10:03 through 10:05
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 6
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	expected := []string{
		"traces_00.json", "traces_01.json", "traces_02.json",
		"traces_03.json", "traces_04.json", "traces_05.json",
	}
	assert.Equal(t, expected, ingestedKeys, "Rolling pass must continue from where backfill finished without partition gaps")
	mu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_Hourly_SlowPass_NoGap(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	var timeMu sync.Mutex
	getNow := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return simulatedTime
	}

	tickerChan := make(chan time.Time, 1)

	var (
		mu            sync.Mutex
		ingestedKeys  []string
		taggedObjects = make(map[string]bool)
		failHour14    atomic.Bool
	)
	failHour14.Store(true)

	key13 := "year=2026/month=11/day=06/hour=13/traces_13.json"
	key14 := "year=2026/month=11/day=06/hour=14/traces_14.json"
	key15 := "year=2026/month=11/day=06/hour=15/traces_15.json"
	key16 := "year=2026/month=11/day=06/hour=16/traces_16.json"
	key17 := "year=2026/month=11/day=06/hour=17/traces_17.json"

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              "year=%Y/month=%m/day=%d/hour=%H",
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Hour,
		lookback:                       1 * time.Hour,
		timeStep:                       time.Hour,
		nowFunc:                        getNow,
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			prefix := *params.Prefix
			if prefix == "year=2026/month=11/day=06/hour=14/traces_" && failHour14.Load() {
				return &mockListObjectsV2Pager{
					Error: errors.New("simulated transient S3 error on hour 14"),
					Pages: []*s3.ListObjectsV2Output{{}},
				}
			}

			var contents []types.Object
			switch prefix {
			case "year=2026/month=11/day=06/hour=13/traces_":
				contents = []types.Object{{Key: &key13}}
			case "year=2026/month=11/day=06/hour=14/traces_":
				contents = []types.Object{{Key: &key14}}
			case "year=2026/month=11/day=06/hour=15/traces_":
				contents = []types.Object{{Key: &key15}}
			case "year=2026/month=11/day=06/hour=16/traces_":
				contents = []types.Object{{Key: &key16}}
			case "year=2026/month=11/day=06/hour=17/traces_":
				contents = []types.Object{{Key: &key17}}
			}

			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				var tags []types.Tag
				if taggedObjects[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				mu.Lock()
				taggedObjects[*params.Key] = true
				mu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			mu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			mu.Unlock()
			return nil
		})
	}()

	// Wait for pass 1 to ingest hour 13 and fail on hour 14
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) == 1 && ingestedKeys[0] == key13
	}, 2*time.Second, 10*time.Millisecond)

	failHour14.Store(false)

	// Advance time to 17:00 (3 hours later > 1h lookback)
	timeMu.Lock()
	simulatedTime = time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	timeMu.Unlock()
	tickerChan <- simulatedTime

	// Wait for pass 2 to retry hour 14 and complete through 17
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 5
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	expected := []string{key13, key14, key15, key16, key17}
	assert.Equal(t, expected, ingestedKeys, "Hourly partition recovery must retry failed hour and cover all elapsed hours without gap")
	mu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}

func Test_s3PollingReader_readAll_CancellationDuringIncompletePass(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	simulatedTime := time.Date(2026, 11, 6, 12, 5, 0, 0, time.UTC)
	key04 := "year=2026/month=11/day=06/hour=12/minute=04/traces_04.json"
	key05 := "year=2026/month=11/day=06/hour=12/minute=05/traces_05.json"

	var mu sync.Mutex
	var firstCallbackHit bool

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Minute,
		lookback:                       2 * time.Minute,
		timeStep:                       time.Minute,
		nowFunc:                        func() time.Time { return simulatedTime },
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return make(chan time.Time), func() {}
		},
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			var contents []types.Object
			switch *params.Prefix {
			case "year=2026/month=11/day=06/hour=12/minute=04/traces_":
				contents = []types.Object{{Key: &key04}}
			case "year=2026/month=11/day=06/hour=12/minute=05/traces_":
				contents = []types.Object{{Key: &key05}}
			}
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, _ *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				return &s3.GetObjectTaggingOutput{TagSet: []types.Tag{}}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, _ *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			mu.Lock()
			if key == key04 {
				firstCallbackHit = true
				cancel() // cancel context during pass
			}
			mu.Unlock()
			return nil
		})
	}()

	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
	mu.Lock()
	assert.True(t, firstCallbackHit)
	mu.Unlock()
}

func Test_s3PollingReader_readAll_LookbackGreaterThanInterval_LateArrival(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	simulatedTime := time.Date(2026, 11, 6, 12, 5, 0, 0, time.UTC)
	var timeMu sync.Mutex
	getNow := func() time.Time {
		timeMu.Lock()
		defer timeMu.Unlock()
		return simulatedTime
	}

	tickerChan := make(chan time.Time, 1)

	var (
		mu            sync.Mutex
		ingestedKeys  []string
		taggedObjects = make(map[string]bool)
		hasLateObject atomic.Bool
	)

	key04 := "year=2026/month=11/day=06/hour=12/minute=04/traces_04.json"
	key05 := "year=2026/month=11/day=06/hour=12/minute=05/traces_05.json"
	keyLate03 := "year=2026/month=11/day=06/hour=12/minute=03/traces_late03.json"
	key06 := "year=2026/month=11/day=06/hour=12/minute=06/traces_06.json"

	reader := &s3PollingReader{
		logger:                         zap.NewNop(),
		s3Bucket:                       "test-bucket",
		s3PartitionFormat:              s3PartitionFormatDefault,
		s3PartitionTimeLocation:        time.UTC,
		filePrefixIncludeTelemetryType: true,
		pollInterval:                   1 * time.Minute,
		lookback:                       3 * time.Minute, // explicit lookback > interval
		timeStep:                       time.Minute,
		nowFunc:                        getNow,
		tickerFunc: func(_ time.Duration) (<-chan time.Time, func()) {
			return tickerChan, func() {}
		},
		tagObjectAfterIngestion:    true,
		skipIngestingTaggedObjects: true,
		listObjectsClient: mockListObjectsAPI(func(params *s3.ListObjectsV2Input) ListObjectsV2Pager {
			var contents []types.Object
			switch *params.Prefix {
			case "year=2026/month=11/day=06/hour=12/minute=03/traces_":
				if hasLateObject.Load() {
					contents = []types.Object{{Key: &keyLate03}}
				}
			case "year=2026/month=11/day=06/hour=12/minute=04/traces_":
				contents = []types.Object{{Key: &key04}}
			case "year=2026/month=11/day=06/hour=12/minute=05/traces_":
				contents = []types.Object{{Key: &key05}}
			case "year=2026/month=11/day=06/hour=12/minute=06/traces_":
				contents = []types.Object{{Key: &key06}}
			}
			return &mockListObjectsV2Pager{
				Pages: []*s3.ListObjectsV2Output{{Contents: contents}},
			}
		}),
		singleObjectClient: &mockSingleObjectAPI{
			getObjectFunc: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte("payload"))),
				}, nil
			},
			getObjectTaggingFunc: func(_ context.Context, params *s3.GetObjectTaggingInput, _ ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error) {
				mu.Lock()
				defer mu.Unlock()
				var tags []types.Tag
				if taggedObjects[*params.Key] {
					tags = []types.Tag{
						{Key: aws.String(ingestedTag), Value: aws.String(ingestedStatus)},
					}
				}
				return &s3.GetObjectTaggingOutput{TagSet: tags}, nil
			},
			putObjectTaggingFunc: func(_ context.Context, params *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
				mu.Lock()
				taggedObjects[*params.Key] = true
				mu.Unlock()
				return &s3.PutObjectTaggingOutput{}, nil
			},
		},
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- reader.readAll(ctx, "traces", func(_ context.Context, key string, _ io.Reader) error {
			mu.Lock()
			ingestedKeys = append(ingestedKeys, key)
			mu.Unlock()
			return nil
		})
	}()

	// Pass 1: covers 12:02 to 12:05 (lookback = 3m)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 2
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []string{key04, key05}, ingestedKeys)
	mu.Unlock()

	// Late arrival appears in minute 03 (2 minutes in the past, within 3m lookback)
	hasLateObject.Store(true)

	// Advance time to 12:06
	timeMu.Lock()
	simulatedTime = simulatedTime.Add(1 * time.Minute)
	timeMu.Unlock()
	tickerChan <- simulatedTime

	// Wait for pass 2: covers 12:03 to 12:06
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ingestedKeys) >= 4
	}, 2*time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Equal(t, []string{key04, key05, keyLate03, key06}, ingestedKeys, "Late arrival within lookback must be ingested without re-ingesting tagged objects")
	mu.Unlock()

	cancel()
	err := <-errChan
	require.ErrorIs(t, err, context.Canceled)
}
