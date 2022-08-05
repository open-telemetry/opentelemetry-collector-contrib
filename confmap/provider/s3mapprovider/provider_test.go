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

package s3mapprovider

import (
	"bytes"
	"context"
	"errors"
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

// A s3 client mocking s3mapprovider works in normal cases
type testClient struct{}

// A provider mocking s3mapprovider works in normal cases
type testProvider struct {
	client testClient
}

// Implement GetObject() for testClient in normal cases
func (client *testClient) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	// read local config file and return
	f, err := ioutil.ReadFile("./testdata/otel-config.yaml")
	if err != nil {
		return &s3.GetObjectOutput{}, err
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
}

// Create a provider mocking s3mapprovider works in normal cases
func NewTestProvider() confmap.Provider {
	return &testProvider{client: testClient{}}
}

func (fp *testProvider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (confmap.Retrieved, error) {
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
	if !errors.Is(err, io.EOF) && err != nil {
		return confmap.Retrieved{}, fmt.Errorf("failed to read content from the downloaded config file via uri %q", uri)
	}

	// unmarshalling the yaml to map[string]interface{}, then construct a Retrieved object
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal(buffer, &rawConf); err != nil {
		return confmap.Retrieved{}, err
	}
	return confmap.NewRetrieved(rawConf)
}

func (fp *testProvider) Scheme() string {
	return schemeName
}

func (fp *testProvider) Shutdown(context.Context) error {
	return nil
}

// A s3 client mocking s3mapprovider works when the returned config file is invalid
type testInvalidClient struct{}

// A provider mocking s3mapprovider works when the returned config file is invalid
type testInvalidProvider struct {
	client testInvalidClient
}

// Create a provider mocking s3mapprovider works when the returned config file is invalid
func NewTestInvalidProvider() confmap.Provider {
	return &testInvalidProvider{client: testInvalidClient{}}
}

// Implement GetObject() for testClient when the returned config file is invalid
func (client *testInvalidClient) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	// read local config file and return
	f := []byte("wrong yaml:[")
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
}

func (fp *testInvalidProvider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (confmap.Retrieved, error) {
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
	if !errors.Is(err, io.EOF) && err != nil {
		return confmap.Retrieved{}, fmt.Errorf("failed to read content from the downloaded config file via uri %q", uri)
	}

	// unmarshalling the yaml to map[string]interface{}, then construct a Retrieved object
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal(buffer, &rawConf); err != nil {
		return confmap.Retrieved{}, err
	}
	return confmap.NewRetrieved(rawConf)
}

func (fp *testInvalidProvider) Scheme() string {
	return schemeName
}

func (fp *testInvalidProvider) Shutdown(context.Context) error {
	return nil
}

// A s3 client mocking s3mapprovider works when there is no corresponding config file according to the given s3-uri
type testNonExistClient struct{}

// A provider mocking s3mapprovider works when there is no corresponding config file according to the given s3-uri
type testNonExistProvider struct {
	client testNonExistClient
}

// Create a provider mocking s3mapprovider works when there is no corresponding config file according to the given s3-uri
func NewTestNonExistProvider() confmap.Provider {
	return &testNonExistProvider{client: testNonExistClient{}}
}

// Implement GetObject() for testClient when there is no corresponding config file according to the given s3-uri
func (client *testNonExistClient) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	// read local config file and return
	f := []byte("wrong yaml:[")
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: (int64)(len(f))}, nil
}

func (fp *testNonExistProvider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (confmap.Retrieved, error) {
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
	if !errors.Is(err, io.EOF) && err != nil {
		return confmap.Retrieved{}, fmt.Errorf("failed to read content from the downloaded config file via uri %q", uri)
	}

	// unmarshalling the yaml to map[string]interface{}, then construct a Retrieved object
	var rawConf map[string]interface{}
	if err := yaml.Unmarshal(buffer, &rawConf); err != nil {
		return confmap.Retrieved{}, err
	}
	return confmap.NewRetrieved(rawConf)
}

func (fp *testNonExistProvider) Scheme() string {
	return schemeName
}

func (fp *testNonExistProvider) Shutdown(context.Context) error {
	return nil
}

func TestFunctionalityDownloadFileS3(t *testing.T) {
	fp := NewTestProvider()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws.com/key", nil)
	assert.NoError(t, err)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestFunctionalityS3URISplit(t *testing.T) {
	fp := NewTestProvider()
	bucket, region, key, err := s3URISplit("s3://bucket.s3.region.amazonaws.com/key")
	assert.NoError(t, err)
	assert.Equal(t, "bucket", bucket)
	assert.Equal(t, "region", region)
	assert.Equal(t, "key", key)
	assert.NoError(t, fp.Shutdown(context.Background()))
}

func TestInvalidS3URISplit(t *testing.T) {
	fp := NewTestProvider()
	_, err := fp.Retrieve(context.Background(), "s3://bucket.s3.region.amazonaws", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(context.Background(), "s3://bucket.s3.region.aws.com/key", nil)
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
