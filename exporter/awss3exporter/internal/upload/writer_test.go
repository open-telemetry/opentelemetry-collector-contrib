// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/tilinna/clock"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestNewS3Manager(t *testing.T) {
	t.Parallel()

	sm := NewS3Manager(
		"my-bucket",
		&PartitionKeyBuilder{},
		s3.New(s3.Options{}),
		"STANDARD",
		WithACL(s3types.ObjectCannedACLPrivate),
	)

	assert.NotNil(t, sm, "Must have a valid client returned")
}

func TestS3ManagerUpload(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		handler      func(t *testing.T) http.Handler
		compression  configcompression.Type
		data         []byte
		errVal       string
		storageClass string
	}{
		{
			name: "successful upload",
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					_ = r.Body.Close()

					assert.Equal(
						t,
						"/my-bucket/telemetry/year=2024/month=01/day=10/hour=10/minute=30/signal-data-noop_random.metrics",
						r.URL.Path,
						"Must match the expected path",
					)
				})
			},
			compression: configcompression.Type(""),
			data:        []byte("hello world"),
			errVal:      "",
		},
		{
			name: "successful compression upload",
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					assert.Equal(
						t,
						"/my-bucket/telemetry/year=2024/month=01/day=10/hour=10/minute=30/signal-data-noop_random.metrics.gz",
						r.URL.Path,
						"Must match the expected path",
					)

					gr, err := gzip.NewReader(r.Body)
					if !assert.NoError(t, err, "Must not error creating gzip reader") {
						return
					}

					data, err := io.ReadAll(gr)
					assert.Equal(t, []byte("hello world"), data, "Must match the expected data")
					assert.NoError(t, err, "Must not error reading data from reader")

					_ = gr.Close()
					_ = r.Body.Close()
				})
			},
			compression: configcompression.TypeGzip,
			data:        []byte("hello world"),
			errVal:      "",
		},
		{
			name: "no data upload",
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					_ = r.Body.Close()

					assert.Fail(t, "Must not call handler when no data is provided")
					w.WriteHeader(http.StatusBadRequest)
				})
			},
			data:   nil,
			errVal: "",
		},
		{
			name: "failed upload",
			handler: func(_ *testing.T) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					_ = r.Body.Close()

					http.Error(w, "Invalid ARN provided", http.StatusUnauthorized)
				})
			},
			data:   []byte("good payload"),
			errVal: "operation error S3: PutObject, https response error StatusCode: 401, RequestID: , HostID: , api error Unauthorized: Unauthorized",
		},
		{
			name: "STANDARD_IA storage class",
			handler: func(t *testing.T) http.Handler {
				return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					// Example of validating that the S3 storage class header is set correctly
					assert.Equal(t, "STANDARD_IA", r.Header.Get("x-amz-storage-class"))
				})
			},
			storageClass: "STANDARD_IA",
			data:         []byte("some data"),
			errVal:       "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := httptest.NewServer(tc.handler(t))
			t.Cleanup(s.Close)

			sm := NewS3Manager(
				"my-bucket",
				&PartitionKeyBuilder{
					PartitionPrefix: "telemetry",
					PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					FilePrefix:      "signal-data-",
					Metadata:        "noop",
					FileFormat:      "metrics",
					Compression:     tc.compression,
					UniqueKeyFunc: func() string {
						return "random"
					},
				},
				s3.New(s3.Options{
					BaseEndpoint: aws.String(s.URL),
					Region:       "local",
				}),
				"STANDARD_IA",
				WithACL(s3types.ObjectCannedACLPrivate),
			)

			// Using a mocked virtual clock to fix the timestamp used
			// to reduce the potential of flaky tests
			mc := clock.NewMock(time.Date(2024, 0o1, 10, 10, 30, 40, 100, time.Local))

			err := sm.Upload(
				clock.Context(context.Background(), mc),
				tc.data,
			)
			if tc.errVal != "" {
				assert.EqualError(t, err, tc.errVal, "Must match the expected error")
			} else {
				assert.NoError(t, err, "Must not have return an error")
			}
		})
	}
}
