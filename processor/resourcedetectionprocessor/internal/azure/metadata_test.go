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

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	assert.NotNil(t, provider)
}

func TestQueryEndpointFailed(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	provider := &azureProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	_, err := provider.Metadata(context.Background())
	assert.Error(t, err)
}

func TestQueryEndpointMalformed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "{")
	}))
	defer ts.Close()

	provider := &azureProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	_, err := provider.Metadata(context.Background())
	assert.Error(t, err)
}

func TestQueryEndpointCorrect(t *testing.T) {
	sentMetadata := &ComputeMetadata{
		Location:          "location",
		Name:              "name",
		VMID:              "vmID",
		VMSize:            "vmSize",
		SubscriptionID:    "subscriptionID",
		ResourceGroupName: "resourceGroup",
	}
	marshalledMetadata, err := json.Marshal(sentMetadata)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(marshalledMetadata)
	}))
	defer ts.Close()

	provider := &azureProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	recvMetadata, err := provider.Metadata(context.Background())

	require.NoError(t, err)
	assert.Equal(t, *sentMetadata, *recvMetadata)
}
