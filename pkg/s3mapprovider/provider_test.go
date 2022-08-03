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
	"io/ioutil"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v2"
)

// Interface testClient standardizes GetObject, which mocks S3 client works
type testClient struct {
	GetObject func(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// Create a client mocking S3 client works in normal cases
func NewTestClient() *testClient {
	return &testClient{
		GetObject: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			// read local config file and return
			f, err := ioutil.ReadFile("./testdata/otel-config.yaml")
			if err != nil {
				return &s3.GetObjectOutput{}, err
			}
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
		}}
}

// testRetrieve: Mock how Retrieve() works in normal cases
type testRetrieve struct {
	client *testClient
}

func NewTestRetrieve() confmap.Provider {
	return &testRetrieve{client: NewTestClient()}
}

func (fp *testRetrieve) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}
	// Split the uri and get [BUCKET], [REGION], [KEY]
	bucket, region, key, err := s3URISplit(uri)
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not valid s3-url", uri)
	}
	// read config file from mocked s3 and return
	resp, err := fp.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(o *s3.Options) {
		o.Region = region
	})
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("file in S3 failed to fetch : uri %q", uri)
	}

	// create a buffer and read content from the response body
	buffer := make([]byte, int(resp.ContentLength))
	defer resp.Body.Close()
	_, err = resp.Body.Read(buffer)
	if err != io.EOF && err != nil {
		return confmap.Retrieved{}, fmt.Errorf("failed to read content from the downloaded config file via uri %q", uri)
	}

	// unmarshalling the yaml to map[string]interface{}, then construct a Retrieved object
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal(buffer, &rawConf); err != nil {
		return confmap.Retrieved{}, err
	}
	return confmap.NewRetrieved(rawConf)
}

func (fp *testRetrieve) Scheme() string {
	return schemeName
}

func (fp *testRetrieve) Shutdown(context.Context) error {
	return nil
}

// Create a client mocking S3 client works when the returned config file is invalid
func NewTestInvalidClient() *testClient {
	return &testClient{
		GetObject: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			// read local config file and return
			f := []byte("wrong yaml:[")
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
		}}
}

// testInvalidRetrieve: Mock how Retrieve() and S3 client works when the returned config file is invalid
type testInvalidRetrieve struct {
	client *testClient
}

func NewTestInvalidRetrieve() confmap.Provider {
	return &testInvalidRetrieve{client: NewTestInvalidClient()}
}

func (fp *testInvalidRetrieve) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}
	// Split the uri and get [BUCKET], [REGION], [KEY]
	bucket, region, key, err := s3URISplit(uri)
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not valid s3-url", uri)
	}
	// read config file from mocked s3 and return
	resp, err := fp.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(o *s3.Options) {
		o.Region = region
	})
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("file in S3 failed to fetch : uri %q", uri)
	}

	// create a buffer and read content from the response body
	buffer := make([]byte, int(resp.ContentLength))
	defer resp.Body.Close()
	_, err = resp.Body.Read(buffer)
	if err != io.EOF && err != nil {
		return confmap.Retrieved{}, fmt.Errorf("failed to read content from the downloaded config file via uri %q", uri)
	}

	// unmarshalling the yaml to map[string]interface{}, then construct a Retrieved object
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal(buffer, &rawConf); err != nil {
		return confmap.Retrieved{}, err
	}
	return confmap.NewRetrieved(rawConf)
}

func (fp *testInvalidRetrieve) Scheme() string {
	return schemeName
}

func (fp *testInvalidRetrieve) Shutdown(context.Context) error {
	return nil
}

// Create a client mocking S3 client works when there is no corresponding config file according to the given s3-uri
func NewTestNonExistClient() *testClient {
	return &testClient{
		GetObject: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			// read local config file and return
			f, err := ioutil.ReadFile("../../testdata/nonexist-config.yaml")
			if err != nil {
				return &s3.GetObjectOutput{}, err
			}
			return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
		}}
}

// testNonExistRetrieve: Mock how Retrieve() works when there is no corresponding config file according to the given s3-uri
type testNonExistRetrieve struct {
	client *testClient
}

func NewTestNonExistRetrieve() confmap.Provider {
	return &testNonExistRetrieve{client: NewTestNonExistClient()}
}

func (fp *testNonExistRetrieve) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}
	// Split the uri and get [BUCKET], [REGION], [KEY]
	bucket, region, key, err := s3URISplit(uri)
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("%q uri is not valid s3-url", uri)
	}
	// read config file from mocked s3 and return
	resp, err := fp.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(o *s3.Options) {
		o.Region = region
	})
	if err != nil {
		return confmap.Retrieved{}, fmt.Errorf("file in S3 failed to fetch : uri %q", uri)
	}

	// create a buffer and read content from the response body
	buffer := make([]byte, int(resp.ContentLength))
	defer resp.Body.Close()
	_, err = resp.Body.Read(buffer)
	if err != io.EOF && err != nil {
		return confmap.Retrieved{}, fmt.Errorf("failed to read content from the downloaded config file via uri %q", uri)
	}

	// unmarshalling the yaml to map[string]interface{}, then construct a Retrieved object
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal(buffer, &rawConf); err != nil {
		return confmap.Retrieved{}, err
	}
	return confmap.NewRetrieved(rawConf)
}

func (fp *testNonExistRetrieve) Scheme() string {
	return schemeName
}

func (fp *testNonExistRetrieve) Shutdown(context.Context) error {
	return nil
}

func TestFunctionalityDownloadFileS3(t *testing.T) {
	fp := NewTestRetrieve()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/key", nil)
	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestFunctionalityS3URISplit(t *testing.T) {
	fp := NewTestRetrieve()
	bucket, region, key, err := s3URISplit("s3://bucket.s3.region.amazonaws.com/key")
	assert.NoError(t, err)
	assert.Equal(t, "bucket", bucket)
	assert.Equal(t, "region", region)
	assert.Equal(t, "key", key)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestInvalidS3URISplit(t *testing.T) {
	fp := NewTestRetrieve()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.region.aws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestUnsupportedScheme(t *testing.T) {
	fp := NewTestRetrieve()
	_, err := fp.Retrieve(context.Background(), "https://google.com", nil)
	assert.Error(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestEmptyBucket(t *testing.T) {
	fp := NewTestRetrieve()
	_, err := fp.Retrieve(context.Background(), "s3://.s3.region.amazonaws.com/key", nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestEmptyKey(t *testing.T) {
	fp := NewTestRetrieve()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/", nil)
	require.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestNonExistent(t *testing.T) {
	fp := NewTestNonExistRetrieve()
	_, err := fp.Retrieve(context.Background(), "s3://non-exist-bucket.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/non-exist-key.yaml", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.non-exist-region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestInvalidYAML(t *testing.T) {
	fp := NewTestInvalidRetrieve()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(context.Background()))
}

func TestScheme(t *testing.T) {
	fp := NewTestRetrieve()
	assert.Equal(t, "s3", fp.Scheme())
	require.NoError(t, fp.Shutdown(context.Background()))
}
