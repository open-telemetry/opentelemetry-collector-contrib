// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsutil

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
)

type fakeClient struct{}

func (f *fakeClient) Get(path string) ([]byte, error) {
	return []byte(path), nil
}

func TestRestClient(t *testing.T) {
	u, _ := url.Parse("http://www.test.com")
	rest, err := NewRestClient(*u, confighttp.HTTPClientSettings{}, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, rest)
}

func TestRestClientFromClient(t *testing.T) {
	rest := NewRestClientFromClient(&fakeClient{})
	metadata, err := rest.GetResponse(endpoints.TaskMetadataPath)

	require.Nil(t, err)
	require.Equal(t, endpoints.TaskMetadataPath, string(metadata))
}

type fakeErrorClient struct{}

func (f *fakeErrorClient) Get(path string) ([]byte, error) {
	return nil, fmt.Errorf("")
}

func TestRestClientError(t *testing.T) {
	rest := NewRestClientFromClient(&fakeErrorClient{})
	metadata, err := rest.GetResponse(endpoints.TaskMetadataPath)

	require.Error(t, err)
	require.Nil(t, metadata)
}

type fakeMetadataErrorClient struct{}

func (f *fakeMetadataErrorClient) Get(path string) ([]byte, error) {
	return nil, fmt.Errorf("")
}

func TestRestClientMetadataError(t *testing.T) {
	rest := NewRestClientFromClient(&fakeMetadataErrorClient{})
	metadata, err := rest.GetResponse(endpoints.TaskMetadataPath)

	require.Error(t, err)
	require.Nil(t, metadata)
}
