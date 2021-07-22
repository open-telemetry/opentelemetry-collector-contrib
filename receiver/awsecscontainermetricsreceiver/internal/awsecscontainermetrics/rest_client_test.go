// Copyright 2020, OpenTelemetry Authors
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

package awsecscontainermetrics

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeClient struct{}

func (f *fakeClient) Get(path string) ([]byte, error) {
	return []byte(path), nil
}

func TestRestClient(t *testing.T) {
	rest := NewRestClient(&fakeClient{})
	stats, metadata, err := rest.EndpointResponse()

	require.Nil(t, err)
	require.Equal(t, taskStatsPath, string(stats))
	require.Equal(t, taskMetadataPath, string(metadata))
}

type fakeErrorClient struct{}

func (f *fakeErrorClient) Get(path string) ([]byte, error) {
	return nil, fmt.Errorf("")
}

func TestRestClientError(t *testing.T) {
	rest := NewRestClient(&fakeErrorClient{})
	stats, metadata, err := rest.EndpointResponse()

	require.Error(t, err)
	require.Nil(t, stats)
	require.Nil(t, metadata)
}

type fakeMetadataErrorClient struct{}

func (f *fakeMetadataErrorClient) Get(path string) ([]byte, error) {
	if path == taskStatsPath {
		return []byte(path), nil
	}
	return nil, fmt.Errorf("")
}

func TestRestClientMetadataError(t *testing.T) {
	rest := NewRestClient(&fakeMetadataErrorClient{})
	stats, metadata, err := rest.EndpointResponse()

	require.Error(t, err)
	require.Nil(t, stats)
	require.Nil(t, metadata)
}
