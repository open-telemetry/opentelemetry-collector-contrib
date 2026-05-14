// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package s3provider

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

// A s3 client mocking s3provider works in normal cases
type testClient struct {
	configFile string
	bucket     string
	region     string
	key        string
}

// Implement GetObject() for testClient in normal cases
func (client *testClient) GetObject(_ context.Context, request *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	s3Opts := s3.Options{}

	for _, opt := range opts {
		opt(&s3Opts)
	}

	client.bucket = *request.Bucket
	client.region = s3Opts.Region
	client.key = *request.Key

	f, err := os.ReadFile(client.configFile)
	if err != nil {
		return &s3.GetObjectOutput{}, err
	}

	bodyLen := int64(len(f))
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f)), ContentLength: &bodyLen}, nil
}

// Create a provider mocking the s3 provider
func newTestProvider(configFile string) confmap.Provider {
	return &provider{client: &testClient{configFile: configFile}}
}

func TestFunctionalityS3URISplit(t *testing.T) {
	fp := newTestProvider("./testdata/otel-config.yaml")
	bucket, region, key, endpoint, err := s3URISplit("s3://bucket.s3.region.amazonaws.com/key")
	assert.NoError(t, err)
	assert.Equal(t, "bucket", bucket)
	assert.Equal(t, "region", region)
	assert.Equal(t, "key", key)
	assert.Empty(t, endpoint)
	assert.NoError(t, fp.Shutdown(t.Context()))
}

func TestFunctionalityS3URISplitCompatible(t *testing.T) {
	fp := newTestProvider("./testdata/otel-config.yaml")
	bucket, region, key, endpoint, err := s3URISplit("s3://minio.example.com/my-bucket/path/to/key.yaml?region=us-east-1")
	assert.NoError(t, err)
	assert.Equal(t, "my-bucket", bucket)
	assert.Equal(t, "us-east-1", region)
	assert.Equal(t, "path/to/key.yaml", key)
	assert.Equal(t, "https://minio.example.com", endpoint)
	assert.NoError(t, fp.Shutdown(t.Context()))
}

func TestFunctionalityS3URISplitCompatibleInsecure(t *testing.T) {
	fp := newTestProvider("./testdata/otel-config.yaml")
	bucket, region, key, endpoint, err := s3URISplit("s3://minio.example.com/my-bucket/config.yaml?insecure=true")
	assert.NoError(t, err)
	assert.Equal(t, "my-bucket", bucket)
	assert.Empty(t, region)
	assert.Equal(t, "config.yaml", key)
	assert.Equal(t, "http://minio.example.com", endpoint)
	assert.NoError(t, fp.Shutdown(t.Context()))
}

func TestURIs(t *testing.T) {
	tests := []struct {
		name   string
		uri    string
		bucket string
		region string
		key    string
		valid  bool
	}{
		{"Invalid domain", "s3://bucket.s3.region.aws.com/key", "", "", "", false},
		{"Invalid region", "s3://bucket.s3.region.aws.amazonaws.com/key", "", "", "", false},
		{"Invalid bucket", "s3://b.s3.region.amazonaws.com/key", "", "", "", false},
		{"No key", "s3://bucket.s3.region.amazonaws.com/", "", "", "", false},
		{"Merged region domain", "s3://bucket.name-here.s3.us-west-2aamazonaws.com/key", "", "", "", false},
		{"No bucket", "s3://s3.region.amazonaws.com/key", "", "", "", false},
		{"No region", "s3://some-bucket.s3..amazonaws.com/key", "", "", "", false},
		{"Test malformed uri", "s3://some-bucket.s3.us-west-2.amazonaws.com/key%", "", "", "", false},
		{"Unsupported scheme", "https://google.com", "", "", "", false},
		{"Valid bucket", "s3://bucket.name-here.s3.us-west-2.amazonaws.com/key", "bucket.name-here", "us-west-2", "key", true},
		// S3-compatible endpoint (path-style) cases
		{"Compatible missing bucket or key", "s3://minio.example.com/onlybucket", "", "", "", false},
		{"Compatible valid with region", "s3://minio.example.com/mybucket/config.yaml?region=us-east-1", "mybucket", "us-east-1", "config.yaml", true},
		{"Compatible no region", "s3://minio.example.com/mybucket/config.yaml", "mybucket", "", "config.yaml", true},
		{"Compatible insecure", "s3://minio.example.com/mybucket/config.yaml?insecure=true", "mybucket", "", "config.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := newTestProvider("./testdata/otel-config.yaml")
			_, err := fp.Retrieve(t.Context(), tt.uri, nil)
			if !tt.valid {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			require.NoError(t, fp.Shutdown(t.Context()))
		})
	}
}

func TestNonExistent(t *testing.T) {
	fp := newTestProvider("./testdata/non-existent.yaml")
	_, err := fp.Retrieve(t.Context(), "s3://non-exist-bucket.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(t.Context(), "s3://bucket.s3.region.amazonaws.com/non-exist-key.yaml", nil)
	assert.Error(t, err)
	_, err = fp.Retrieve(t.Context(), "s3://bucket.s3.non-exist-region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(t.Context()))
}

func TestInvalidYAML(t *testing.T) {
	fp := newTestProvider("./testdata/invalid-otel-config.yaml")
	_, err := fp.Retrieve(t.Context(), "s3://bucket.s3.region.amazonaws.com/key", nil)
	assert.Error(t, err)
	require.NoError(t, fp.Shutdown(t.Context()))
}

func TestScheme(t *testing.T) {
	fp := newTestProvider("./testdata/otel-config.yaml")
	assert.Equal(t, "s3", fp.Scheme())
	require.NoError(t, fp.Shutdown(t.Context()))
}

func TestFactory(t *testing.T) {
	p := NewFactory().Create(confmap.ProviderSettings{})
	_, ok := p.(*provider)
	require.True(t, ok)
}
