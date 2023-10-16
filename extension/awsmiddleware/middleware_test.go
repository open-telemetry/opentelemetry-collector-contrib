// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"
	awsv1 "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/awstesting"
	s3v1 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUserAgent = "user/agent"
	testLatency   = time.Millisecond
)

type testHandler struct {
	id             string
	position       HandlerPosition
	handleRequest  func(r *http.Request)
	handleResponse func(r *http.Response)
	start          time.Time
	end            time.Time
}

var _ RequestHandler = (*testHandler)(nil)
var _ ResponseHandler = (*testHandler)(nil)

func (t *testHandler) ID() string {
	return t.id
}

func (t *testHandler) Position() HandlerPosition {
	return t.position
}

func (t *testHandler) HandleRequest(r *http.Request) {
	t.start = time.Now()
	if t.handleRequest != nil {
		t.handleRequest(r)
	}
}

func (t *testHandler) HandleResponse(r *http.Response) {
	t.end = time.Now()
	if t.handleResponse != nil {
		t.handleResponse(r)
	}
}

func (t *testHandler) Latency() time.Duration {
	return t.end.Sub(t.start)
}

type recordOrder struct {
	order []string
}

func (ro *recordOrder) handle(id string) func(*http.Request) {
	return func(*http.Request) {
		ro.order = append(ro.order, id)
	}
}

func TestHandlerPosition(t *testing.T) {
	testCases := []struct {
		position HandlerPosition
		str      string
	}{
		{position: After, str: "after"},
		{position: Before, str: "before"},
	}
	for _, testCase := range testCases {
		position := testCase.position
		got, err := position.MarshalText()
		assert.NoError(t, err)
		assert.EqualValues(t, testCase.str, got)
		assert.NoError(t, position.UnmarshalText(got))
		assert.Equal(t, position, testCase.position)
	}
}

func TestInvalidHandlerPosition(t *testing.T) {
	position := HandlerPosition(-1)
	got, err := position.MarshalText()
	assert.Error(t, err)
	assert.ErrorIs(t, err, errUnsupportedPosition)
	assert.Nil(t, got)
	err = position.UnmarshalText([]byte("HandlerPosition(-1)"))
	assert.Error(t, err)
	assert.ErrorIs(t, err, errUnsupportedPosition)
}

func TestInvalidHandlers(t *testing.T) {
	invalidHandler := &testHandler{id: "invalid handler", position: -1}
	testExtension := &testMiddlewareExtension{
		requestHandlers:  []RequestHandler{invalidHandler},
		responseHandlers: []ResponseHandler{invalidHandler},
	}
	// v1
	client := awstesting.NewClient()
	err := ConfigureSDKv1(testExtension, &client.Handlers)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errInvalidHandler))
	assert.True(t, errors.Is(err, errUnsupportedPosition))
	// v2
	err = ConfigureSDKv2(testExtension, &awsv2.Config{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errInvalidHandler))
	assert.True(t, errors.Is(err, errUnsupportedPosition))
}

func TestAppendOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	testCases := map[string]struct {
		requestHandlers []*testHandler
		wantOrder       []string
	}{
		"WithBothBefore": {
			requestHandlers: []*testHandler{
				{id: "1", position: Before},
				{id: "2", position: Before},
			},
			wantOrder: []string{"2", "1"},
		},
		"WithBothAfter": {
			requestHandlers: []*testHandler{
				{id: "1", position: After},
				{id: "2", position: After},
			},
			wantOrder: []string{"1", "2"},
		},
		"WithBeforeAfter": {
			requestHandlers: []*testHandler{
				{id: "1", position: Before},
				{id: "2", position: After},
			},
			wantOrder: []string{"1", "2"},
		},
		"WithAfterBefore": {
			requestHandlers: []*testHandler{
				{id: "1", position: After},
				{id: "2", position: Before},
			},
			wantOrder: []string{"2", "1"},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			middleware := &testMiddlewareExtension{}
			recorder := &recordOrder{}
			for _, handler := range testCase.requestHandlers {
				handler.handleRequest = recorder.handle(handler.id)
				middleware.requestHandlers = append(middleware.requestHandlers, handler)
			}
			// v1
			client := awstesting.NewClient(&awsv1.Config{
				Region:     awsv1.String("mock-region"),
				DisableSSL: awsv1.Bool(true),
				Endpoint:   awsv1.String(server.URL),
			})
			assert.NoError(t, ConfigureSDKv1(middleware, &client.Handlers))
			s3v1Client := &s3v1.S3{Client: client}
			_, err := s3v1Client.ListBuckets(&s3v1.ListBucketsInput{})
			require.NoError(t, err)
			assert.Equal(t, testCase.wantOrder, recorder.order)
			recorder.order = nil
			// v2
			cfg := awsv2.Config{Region: "us-east-1"}
			assert.NoError(t, ConfigureSDKv2(middleware, &cfg))
			s3v2Client := s3v2.NewFromConfig(cfg, func(options *s3v2.Options) {
				options.BaseEndpoint = awsv2.String(server.URL)
			})
			_, err = s3v2Client.ListBuckets(context.Background(), &s3v2.ListBucketsInput{})
			require.NoError(t, err)
			assert.Equal(t, testCase.wantOrder, recorder.order)
		})
	}
}

func TestConfigureSDKv1(t *testing.T) {
	middleware, recorder, server := setup(t)
	defer server.Close()
	client := awstesting.NewClient(&awsv1.Config{
		Region:     awsv1.String("mock-region"),
		DisableSSL: awsv1.Bool(true),
		Endpoint:   awsv1.String(server.URL),
	})
	require.Equal(t, 3, client.Handlers.Build.Len())
	require.Equal(t, 0, client.Handlers.Unmarshal.Len())
	assert.NoError(t, ConfigureSDKv1(middleware, &client.Handlers))
	assert.Equal(t, 5, client.Handlers.Build.Len())
	assert.Equal(t, 1, client.Handlers.Unmarshal.Len())
	s3Client := &s3v1.S3{Client: client}
	output, err := s3Client.ListBuckets(&s3v1.ListBucketsInput{})
	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.GreaterOrEqual(t, recorder.Latency(), testLatency)
}

func TestConfigureSDKv2(t *testing.T) {
	middleware, recorder, server := setup(t)
	defer server.Close()
	cfg := awsv2.Config{Region: "us-east-1"}
	assert.NoError(t, ConfigureSDKv2(middleware, &cfg))
	s3Client := s3v2.NewFromConfig(cfg, func(options *s3v2.Options) {
		options.BaseEndpoint = awsv2.String(server.URL)
	})
	output, err := s3Client.ListBuckets(context.Background(), &s3v2.ListBucketsInput{})
	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.GreaterOrEqual(t, recorder.Latency(), testLatency)
}

func setup(t *testing.T) (Middleware, *testHandler, *httptest.Server) {
	t.Helper()
	recorder := &testHandler{id: "LatencyTest", position: After}
	middleware := &testMiddlewareExtension{
		requestHandlers: []RequestHandler{
			&testHandler{
				id:       "UserAgentTest",
				position: Before,
				handleRequest: func(r *http.Request) {
					r.Header.Set("User-Agent", testUserAgent)
				},
			},
			recorder,
		},
		responseHandlers: []ResponseHandler{recorder},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent := r.Header.Get("User-Agent")
		assert.Contains(t, gotUserAgent, testUserAgent)
		time.Sleep(testLatency)
		w.WriteHeader(http.StatusOK)
	}))
	return middleware, recorder, server
}
