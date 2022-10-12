// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package s3provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

// testClient is an s3 client mocking s3provider works in normal cases
type testClient struct{}

// NewTestProvider returns a mock provider for "normal working" tests
func NewTestProvider() confmap.Provider {
	return &provider{client: &testClient{}}
}

// GetObject implements s3client for `testClient`
func (client *testClient) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	// read local config file and return
	f, err := os.ReadFile("./testdata/otel-config.yaml")
	if err != nil {
		return nil, err
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
}

// A s3 client mocking s3provider works when there is no corresponding config file according to the given s3-uri
type testNonExistClient struct{}

// NewTestNonExistProvider returns a mock provider for "non-existing configuration file" tests
func NewTestNonExistProvider() confmap.Provider {
	return &provider{client: &testNonExistClient{}}
}

// GetObject implements s3client for `testNonExistClient`
func (client *testNonExistClient) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	// read local config file and return
	f, err := os.ReadFile("./testdata/nonexist-otel-config.yaml")
	if err != nil {
		return nil, err
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
}

// A s3 client mocking s3provider works when the fetch fails
type testInvalidFetchClient struct{}

// NewTestInvalidFetchProvider returns a mock provider for "s3 fetch failed" tests
func NewTestInvalidFetchProvider() confmap.Provider {
	return &provider{client: &testInvalidFetchClient{}}
}

// GetObject implements s3client for `testInvalidFetchClient`
func (client *testInvalidFetchClient) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return nil, fmt.Errorf("network error")
}

// A s3 client mocking s3provider works when the returned config file is invalid
type testInvalidClient struct{}

// NewTestInvalidProvider returns a mock provider for "invalid configuration file" tests
func NewTestInvalidProvider() confmap.Provider {
	return &provider{client: &testInvalidClient{}}
}

// GetObject implements s3client for `testInvalidClient`
func (client *testInvalidClient) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	buff := new(bytes.Buffer)
	_, _ = buff.Write([]byte("invalid yaml"))
	return &s3.GetObjectOutput{Body: io.NopCloser(buff), ContentLength: (int64)(buff.Len())}, nil
}

func TestFunctionalityDownloadFileS3(t *testing.T) {
	fp := NewTestProvider()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/key", nil)
	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestSplitS3URI(t *testing.T) {
	fp := NewTestProvider()
	bucket, region, key, err := splitS3URI("s3://bucket.s3.region.amazonaws.com/key")
	assert.NoError(t, err)
	assert.Equal(t, "bucket", bucket)
	assert.Equal(t, "region", region)
	assert.Equal(t, "key", key)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestSplitS3URI_Invalid(t *testing.T) {
	fp := NewTestProvider()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.region.aws.com/key", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.co.uk/key", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://h?.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://h^.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))

}

func TestUnsupportedScheme(t *testing.T) {
	fp := NewTestProvider()
	_, err := fp.Retrieve(context.Background(), "https://google.com", nil)
	assert.Error(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestEmptyBucket(t *testing.T) {
	fp := NewTestProvider()
	_, err := fp.Retrieve(context.Background(), "s3://.s3.region.amazonaws.com/key", nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestEmptyKey(t *testing.T) {
	fp := NewTestProvider()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/", nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestNonExistent(t *testing.T) {
	fp := NewTestNonExistProvider()
	_, err := fp.Retrieve(context.Background(), "s3://non-exist-bucket.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/non-exist-key.yaml", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.non-exist-region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestS3FetchFail(t *testing.T) {
	fp := NewTestInvalidFetchProvider()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestInvalidYAML(t *testing.T) {
	fp := NewTestInvalidProvider()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestScheme(t *testing.T) {
	fp := NewTestProvider()
	assert.Equal(t, "s3", fp.Scheme())
	require.NoError(t, fp.Shutdown(context.Background()))
}
