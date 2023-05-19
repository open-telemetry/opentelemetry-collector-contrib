// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
		_, err := fmt.Fprintln(w, "{")
		assert.NoError(t, err)
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
		_, err = w.Write(marshalledMetadata)
		assert.NoError(t, err)
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
