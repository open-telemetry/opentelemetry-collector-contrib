// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
