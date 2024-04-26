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
	case "RetrieveProperties":
		return routeRetreiveProperties(t, body)
	case "QueryPerf":
		return routePerformanceQuery(t, body)
	case "CreateContainerView":
		return loadResponse("create-container-view.xml")
	}

	return []byte{}, errNotFound
}

func routeRetreiveProperties(t *testing.T, body map[string]any) ([]byte, error) {
	rp, ok := body["RetrieveProperties"].(map[string]any)
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
		content = obj["#content"].(string)
		contentType = obj["-type"].(string)
	}

	switch {
	case content == "group-d1" && contentType == "Folder":
		for _, i := range propSetArray {
			m, ok := i.(map[string]any)
			require.True(t, ok)
			if m["pathSet"] == "parentVApp" && m["type"] == "VirtualMachine" {
				return loadResponse("datacenter-list.xml")
			}
		}
		return loadResponse("datacenter.xml")

	case content == "datacenter-3" && contentType == "Datacenter":
		return loadResponse("datacenter-properties.xml")

	case content == "datastore-1003" && contentType == "Datastore":
		if objectSetArray {
			return loadResponse("datastore-list.xml")
		}
		return loadResponse("datastore-properties.xml")

	case content == "domain-c8" && contentType == "ClusterComputeResource":
		for _, prop := range propSetArray {
			spec := prop.(map[string]any)
			specType := spec["type"].(string)
			if specType == "ResourcePool" {
				return loadResponse("cluster-children.xml")
			}
		}
		path := propSet["pathSet"].(string)
		switch path {
		case "datastore":
			return loadResponse("cluster-datastore.xml")
		case "summary":
			return loadResponse("cluster-summary.xml")
		case "host":
			return loadResponse("cluster-host.xml")
		}

	case content == "domain-c9" && contentType == "ComputeResource":
		for _, prop := range propSetArray {
			spec := prop.(map[string]any)
			specType := spec["type"].(string)
			if specType == "ResourcePool" {
				return loadResponse("compute-children.xml")
			}
		}
		path := propSet["pathSet"].(string)
		switch path {
		case "datastore":
			return loadResponse("compute-datastore.xml")
		case "host":
			return loadResponse("compute-host.xml")
		}

	case contentType == "ResourcePool":
		if ps, ok := propSet["pathSet"].([]any); ok {
			for _, prop := range ps {
				if prop == "summary" {
					if content == "resgroup-9" {
						return loadResponse("cluster-resource-pool-properties.xml")
					}
					if content == "resgroup-10" {
						return loadResponse("compute-resource-pool-properties.xml")
					}
				}
			}
		}
		if ps, ok := propSet["pathSet"].(string); ok {
			if ps == "owner" {
				if content == "resgroup-9" {
					return loadResponse("cluster-resource-pool-owner.xml")
				}
				if content == "resgroup-10" {
					return loadResponse("compute-resource-pool-owner.xml")
				}
			}
		}
		if ss, ok := objectSet["selectSet"].(map[string]any); ok && ss["path"] == "resourcePool" {
			if content == "resgroup-9" {
				return loadResponse("retrieve-properties-empty.xml")
			}
			if content == "resgroup-10" {
				return loadResponse("retrieve-properties-empty.xml")
			}
		}

	case content == "resgroup-v10" && contentType == "VirtualApp":
		for _, prop := range propSetArray {
			innerPropSet, ok := prop.(map[string]any)
			require.True(t, ok)
			if innerPropSet["type"] == "VirtualMachine" {
				return loadResponse("virtual-app-children.xml")
			}
		}
		if _, ok := propSet["pathSet"].([]any); ok {
			return loadResponse("virtual-app-properties.xml")
		}
		if ps, ok := propSet["pathSet"].(string); ok {
			if ps == "owner" {
				return loadResponse("virtual-app-owner.xml")
			}
		}

	case content == "group-h5" && contentType == "Folder":
		for _, i := range propSetArray {
			m, ok := i.(map[string]any)
			require.True(t, ok)
			if m["type"] == "ClusterComputeResource" {
				return loadResponse("host-folder-children.xml")
			}
		}
		return loadResponse("host-folder-parent.xml")

	case contentType == "HostSystem":
		if ps, ok := propSet["pathSet"].([]any); ok {
			for _, v := range ps {
				if v == "summary.hardware" || v == "summary.hardware.cpuMhz" {
					if content == "host-1002" {
						return loadResponse("cluster-host-properties.xml")
					}
					if content == "host-1003" {
						return loadResponse("compute-host-properties.xml")
					}
				}
			}
		} else {
			ps, ok := propSet["pathSet"].(string)
			require.True(t, ok)
			if ps == "name" {
				if content == "host-1002" {
					return loadResponse("cluster-host-name.xml")
				}
				if content == "host-1003" {
					return loadResponse("compute-host-name.xml")
				}
			}
			if ps == "summary.hardware" {
				if content == "host-1002" {
					return loadResponse("cluster-host-properties.xml")
				}
				if content == "host-1003" {
					return loadResponse("compute-host-properties.xml")
				}
			}
		}

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

	case contentType == "ContainerView" && propSet["type"] == "VirtualMachine":
		return loadResponse("vm-default-properties.xml")

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
		if entity["#content"] == "host-1002" {
			return loadResponse("cluster-host-perf-counters.xml")
		}
		if entity["#content"] == "host-1003" {
			return loadResponse("compute-host-perf-counters.xml")
		}
	case "VirtualMachine":
		return loadResponse("vm-performance-counters.xml")
	}
	return []byte{}, errNotFound
}

func loadResponse(filename string) ([]byte, error) {
	return os.ReadFile(filepath.Join("internal", "mockserver", "responses", filename))
}
