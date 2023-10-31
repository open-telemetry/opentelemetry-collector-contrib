// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	provider1 := NewProvider("127.0.0.1:4444", "abc", nil)
	assert.NotNil(t, provider1)
	provider2 := NewProvider("", "", nil)
	assert.NotNil(t, provider2)
}

func TestQueryEndpointFailed(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	_, err := provider.OpenShiftClusterVersion(context.Background())
	assert.Error(t, err)

	_, err = provider.K8SClusterVersion(context.Background())
	assert.Error(t, err)

	_, err = provider.Infrastructure(context.Background())
	assert.Error(t, err)
}

func TestQueryEndpointMalformed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintln(w, "{")
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "",
		client:  &http.Client{},
	}

	_, err := provider.Infrastructure(context.Background())
	assert.Error(t, err)
}

func TestQueryEndpointCorrectK8SClusterVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
  "major": "1",
  "minor": "21",
  "gitVersion": "v1.21.11+5cc9227",
  "gitCommit": "047f86f8e2212f25394de1c8bad35d9426ae0f4c",
  "gitTreeState": "clean",
  "buildDate": "2022-09-20T16:39:45Z",
  "goVersion": "go1.16.12",
  "compiler": "gc",
  "platform": "linux/amd64"
}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	got, err := provider.K8SClusterVersion(context.Background())
	require.NoError(t, err)
	expect := "v1.21.11"
	assert.Equal(t, expect, got)
}

func TestQueryEndpointCorrectOpenShiftClusterVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
"apiVersion": "config.openshift.io/v1",
"kind": "ClusterVersion",
"status": {
  "desired": {"version": "4.8.51"}
  }
}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	got, err := provider.OpenShiftClusterVersion(context.Background())
	require.NoError(t, err)
	expect := "4.8.51"
	assert.Equal(t, expect, got)
}

func TestQueryEndpointCorrectInfrastructureAWS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
"apiVersion": "config.openshift.io/v1",
"kind": "Infrastructure",
"status": {
  "apiServerInternalURI": "https://api.myopenshift.com:4443",
  "apiServerURL": "https://api.myopenshift.com:4443",
  "controlPlaneTopology": "HighlyAvailable",
  "etcdDiscoveryDomain": "",
  "infrastructureName": "test-d-bm4rt",
  "infrastructureTopology": "HighlyAvailable",
  "platform": "AWS",
  "platformStatus": {
	"type": "AWS",
	"aws": {"region": "us-east-1"}
}}}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	got, err := provider.Infrastructure(context.Background())
	require.NoError(t, err)
	expect := InfrastructureAPIResponse{
		Status: InfrastructureStatus{
			InfrastructureName:     "test-d-bm4rt",
			ControlPlaneTopology:   "HighlyAvailable",
			InfrastructureTopology: "HighlyAvailable",
			PlatformStatus: InfrastructurePlatformStatus{
				Type: "AWS",
				Aws: InfrastructureStatusAWS{
					Region: "us-east-1",
				},
			},
		},
	}
	assert.Equal(t, expect, *got)
}

func TestQueryEndpointCorrectInfrastructureAzure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
"apiVersion": "config.openshift.io/v1",
"kind": "Infrastructure",
"status": {
  "apiServerInternalURI": "https://api.myopenshift.com:4443",
  "apiServerURL": "https://api.myopenshift.com:4443",
  "controlPlaneTopology": "HighlyAvailable",
  "etcdDiscoveryDomain": "",
  "infrastructureName": "test-d-bm4rt",
  "infrastructureTopology": "HighlyAvailable",
  "platform": "AZURE",
  "platformStatus": {
	"type": "AZURE",
	"azure": {"cloudName": "us-east-1"}
}}}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	got, err := provider.Infrastructure(context.Background())
	require.NoError(t, err)
	expect := InfrastructureAPIResponse{
		Status: InfrastructureStatus{
			InfrastructureName:     "test-d-bm4rt",
			ControlPlaneTopology:   "HighlyAvailable",
			InfrastructureTopology: "HighlyAvailable",
			PlatformStatus: InfrastructurePlatformStatus{
				Type: "AZURE",
				Azure: InfrastructureStatusAzure{
					CloudName: "us-east-1",
				},
			},
		},
	}
	assert.Equal(t, expect, *got)
}

func TestQueryEndpointCorrectInfrastructureGCP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
"apiVersion": "config.openshift.io/v1",
"kind": "Infrastructure",
"status": {
  "controlPlaneTopology": "HighlyAvailable",
  "etcdDiscoveryDomain": "",
  "infrastructureName": "test-d-bm4rt",
  "infrastructureTopology": "HighlyAvailable",
  "platform": "GCP",
  "platformStatus": {
	"type": "GCP",
	"gcp": {"region": "us-east-1"}
}}}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	got, err := provider.Infrastructure(context.Background())
	require.NoError(t, err)
	expect := InfrastructureAPIResponse{
		Status: InfrastructureStatus{
			InfrastructureName:     "test-d-bm4rt",
			ControlPlaneTopology:   "HighlyAvailable",
			InfrastructureTopology: "HighlyAvailable",
			PlatformStatus: InfrastructurePlatformStatus{
				Type: "GCP",
				GCP: InfrastructureStatusGCP{
					Region: "us-east-1",
				},
			},
		},
	}
	assert.Equal(t, expect, *got)
}

func TestQueryEndpointCorrectInfrastructureIBMCloud(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
"apiVersion": "config.openshift.io/v1",
"kind": "Infrastructure",
"status": {
  "controlPlaneTopology": "HighlyAvailable",
  "etcdDiscoveryDomain": "",
  "infrastructureName": "test-d-bm4rt",
  "infrastructureTopology": "HighlyAvailable",
  "platform": "IBMCloud",
  "platformStatus": {
	"type": "ibmcloud",
	"ibmcloud": {"location": "us-east-1"}
}}}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	got, err := provider.Infrastructure(context.Background())
	require.NoError(t, err)
	expect := InfrastructureAPIResponse{
		Status: InfrastructureStatus{
			InfrastructureName:     "test-d-bm4rt",
			ControlPlaneTopology:   "HighlyAvailable",
			InfrastructureTopology: "HighlyAvailable",
			PlatformStatus: InfrastructurePlatformStatus{
				Type: "ibmcloud",
				IBMCloud: InfrastructureStatusIBMCloud{
					Location: "us-east-1",
				},
			},
		},
	}
	assert.Equal(t, expect, *got)
}

func TestQueryEndpointCorrectInfrastructureOpenstack(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, `{
"apiVersion": "config.openshift.io/v1",
"kind": "Infrastructure",
"status": {
  "controlPlaneTopology": "HighlyAvailable",
  "etcdDiscoveryDomain": "",
  "infrastructureName": "test-d-bm4rt",
  "infrastructureTopology": "HighlyAvailable",
  "platform": "openstack",
  "platformStatus": {
	"type": "openstack",
	"openstack": {"cloudName": "us-east-1"}
}}}`)
		assert.NoError(t, err)
	}))
	defer ts.Close()

	provider := &openshiftProvider{
		address: ts.URL,
		token:   "test",
		client:  &http.Client{},
	}

	got, err := provider.Infrastructure(context.Background())
	require.NoError(t, err)
	expect := InfrastructureAPIResponse{
		Status: InfrastructureStatus{
			InfrastructureName:     "test-d-bm4rt",
			ControlPlaneTopology:   "HighlyAvailable",
			InfrastructureTopology: "HighlyAvailable",
			PlatformStatus: InfrastructurePlatformStatus{
				Type: "openstack",
				OpenStack: InfrastructureStatusOpenStack{
					CloudName: "us-east-1",
				},
			},
		},
	}
	assert.Equal(t, expect, *got)
}
