// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mockserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/mockserver"

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	xj "github.com/basgys/goxml2json"
	"github.com/stretchr/testify/require"
)

const (
	// MockUsername is the correct user for authentication to the Mock Server
	MockUsername = "otelu"
	// MockPassword is the correct password for authentication to the Mock Server
	MockPassword = "otelp"
)

var errNotFound = errors.New("not found")

type soapRequest struct {
	Envelope soapEnvelope `json:"Envelope"`
}

type soapEnvelope struct {
	Body map[string]any `json:"Body"`
}

// MockServer has access to recorded SOAP responses and will serve them over http based off the scraper's API calls
func MockServer(t *testing.T, useTLS bool) *httptest.Server {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// converting to JSON in order to iterate over map keys
		jsonified, err := xj.Convert(r.Body)
		require.NoError(t, err)
		sr := &soapRequest{}
		err = json.Unmarshal(jsonified.Bytes(), sr)
		require.NoError(t, err)
		require.Len(t, sr.Envelope.Body, 1)

		var requestType string
		for k := range sr.Envelope.Body {
			requestType = k
		}
		require.NotEmpty(t, requestType)

		body, err := routeBody(t, requestType, sr.Envelope.Body)
		if errors.Is(err, errNotFound) {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write(body)
	})

	if useTLS {
		return httptest.NewTLSServer(handlerFunc)
	}

	return httptest.NewServer(handlerFunc)
}

func routeBody(t *testing.T, requestType string, body map[string]any) ([]byte, error) {
	switch requestType {
	case "RetrieveServiceContent":
		return loadResponse("service-content.xml")
	case "Login":
		return loadResponse("login.xml")
	case "Logout":
		return loadResponse("logout.xml")
	case "RetrievePropertiesEx":
		return routeRetreivePropertiesEx(t, body)
	case "QueryPerf":
		return routePerformanceQuery(t, body)
	case "CreateContainerView":
		return loadResponse("create-container-view.xml")
	case "DestroyView":
		return loadResponse("destroy-view.xml")
	case "VsanPerfQueryPerf":
		return routeVsanPerfQueryPerf(t, body)
	}

	return []byte{}, errNotFound
}

func routeRetreivePropertiesEx(t *testing.T, body map[string]any) ([]byte, error) {
	rp, ok := body["RetrievePropertiesEx"].(map[string]any)
	require.True(t, ok)
	specSet := rp["specSet"].(map[string]any)

	var objectSetArray = false
	objectSet, ok := specSet["objectSet"].(map[string]any)
	if !ok {
		objectSetArray = true
	}

	propSet, ok := specSet["propSet"].(map[string]any)
	var propSetArray []any
	if !ok {
		propSetArray = specSet["propSet"].([]any)
	}

	var obj map[string]any
	var content string
	var contentType string
	if !objectSetArray {
		obj = objectSet["obj"].(map[string]any)
		if value, exists := obj["#content"]; exists {
			content = value.(string)
		} else {
			content = ""
		}
		contentType = obj["-type"].(string)
	}

	switch {
	case content == "group-d1" && contentType == "Folder":
		for _, i := range propSetArray {
			m, ok := i.(map[string]any)
			require.True(t, ok)
			if m["type"] == "Folder" {
				return loadResponse("datacenter.xml")
			}
		}
		return loadResponse("datacenter-folder.xml")

	case content == "datacenter-3" && contentType == "Datacenter":
		return loadResponse("datacenter-properties.xml")

	case content == "domain-c8" && contentType == "ClusterComputeResource":
		return loadResponse("cluster-children.xml")

	case content == "domain-c9" && contentType == "ComputeResource":
		return loadResponse("compute-children.xml")

	case contentType == "ResourcePool":
		return loadResponse("retrieve-properties-empty.xml")

	case content == "group-h5" && contentType == "Folder":
		for _, i := range propSetArray {
			m, ok := i.(map[string]any)
			require.True(t, ok)
			if m["type"] == "ClusterComputeResource" {
				return loadResponse("host-folder-children.xml")
			}
		}
		return loadResponse("host-folder-parent.xml")

	case content == "group-v4" && contentType == "Folder":
		for _, i := range propSetArray {
			m, ok := i.(map[string]any)
			require.True(t, ok)
			if m["pathSet"] == "parentVApp" && m["type"] == "VirtualMachine" {
				return loadResponse("vm-folder-parents.xml")
			}
		}
		return loadResponse("vm-folder-children.xml")

	case (content == "group-v1034" || content == "group-v1001") && contentType == "Folder":
		return loadResponse("retrieve-properties-empty.xml")

	case contentType == "ContainerView":
		if propSet["type"] == "Datacenter" {
			return loadResponse("datacenter.xml")
		}
		if propSet["type"] == "Datastore" {
			return loadResponse("datastore-default-properties.xml")
		}
		if propSet["type"] == "ComputeResource" {
			return loadResponse("compute-default-properties.xml")
		}
		if propSet["type"] == "HostSystem" {
			return loadResponse("host-default-properties.xml")
		}
		if propSet["type"] == "ResourcePool" {
			return loadResponse("resource-pool-default-properties.xml")
		}
		if propSet["type"] == "VirtualMachine" {
			return loadResponse("vm-default-properties.xml")
		}

	case content == "PerfMgr" && contentType == "PerformanceManager":
		return loadResponse("perf-manager.xml")
	}

	return []byte{}, errNotFound
}

func routePerformanceQuery(t *testing.T, body map[string]any) ([]byte, error) {
	queryPerf := body["QueryPerf"].(map[string]any)
	require.NotNil(t, queryPerf)
	querySpec, ok := queryPerf["querySpec"].(map[string]any)
	if !ok {
		querySpecs := queryPerf["querySpec"].([]any)
		querySpec = querySpecs[0].(map[string]any)
	}
	entity := querySpec["entity"].(map[string]any)
	switch entity["-type"] {
	case "HostSystem":
		return loadResponse("host-performance-counters.xml")
	case "VirtualMachine":
		return loadResponse("vm-performance-counters.xml")
	}
	return []byte{}, errNotFound
}

func routeVsanPerfQueryPerf(t *testing.T, body map[string]any) ([]byte, error) {
	queryPerf := body["VsanPerfQueryPerf"].(map[string]any)
	require.NotNil(t, queryPerf)
	querySpecs, ok := queryPerf["querySpecs"].(map[string]any)
	if !ok {
		return []byte{}, errNotFound
	}
	entityRefID := querySpecs["entityRefId"].(string)
	switch entityRefID {
	case "cluster-domclient:*":
		return loadResponse("cluster-vsan.xml")
	case "host-domclient:*":
		return loadResponse("host-vsan.xml")
	case "virtual-machine:*":
		return loadResponse("vm-vsan.xml")
	default:
		return []byte{}, errNotFound
	}
}

func loadResponse(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join("internal", "mockserver", "responses", filename))
}
