// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewProvider verifies that NewProvider returns a non-nil provider.
func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	assert.NotNil(t, provider)
}

// TestQueryEndpointFailed ensures that the provider returns an error
// when the OracleCloud IMDS endpoint replies with a non-OK status.
func TestQueryEndpointFailed(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	provider := &oraclecloudProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	_, err := provider.Metadata(t.Context())
	assert.Error(t, err)
}

// TestQueryEndpointMalformed ensures the provider returns an error
// when the OracleCloud IMDS endpoint returns malformed JSON.
func TestQueryEndpointMalformed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintln(w, "{")
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &oraclecloudProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	_, err := provider.Metadata(t.Context())
	assert.Error(t, err)
}

// TestQueryEndpointCorrect validates that the provider correctly retrieves
// and parses the metadata from a well-formed IMDS endpoint.
func TestQueryEndpointCorrect(t *testing.T) {
	sentMetadata := &ComputeMetadata{
		HostID:             "ocid1.instance.oc1..aaaaaaa",
		HostDisplayName:    "my-instance",
		HostType:           "VM.Standard.E4.Flex",
		RegionID:           "us-ashburn-1",
		AvailabilityDomain: "AD-1",
		Metadata: InstanceMetadata{
			OKEClusterDisplayName: "my-oke-cluster",
		},
	}
	marshalledMetadata, err := json.Marshal(sentMetadata)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err = w.Write(marshalledMetadata)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &oraclecloudProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	recvMetadata, err := provider.Metadata(t.Context())

	require.NoError(t, err)
	assert.Equal(t, *sentMetadata, *recvMetadata)
}
