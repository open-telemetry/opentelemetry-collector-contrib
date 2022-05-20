// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mockserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/mockserver"

import (
	"embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	// MockUser500Response will cause the MockServer to return a 500 response code
	MockUser500Response = "500user"
)

var errNotFound = errors.New("not found")

var responses embed.FS

type soapRequest struct {
	Envelope soapEnvelope `json:"Envelope"`
}

type soapEnvelope struct {
	Body map[string]interface{} `json:"Body"`
}

// MockServer has access to recorded SOAP responses and will serve them over http based off the scraper's API calls
func MockServer(t *testing.T, responsesFolder embed.FS) *httptest.Server {
	responses = responsesFolder
	vsphereMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authUser, _, _ := r.BasicAuth()
		if authUser == MockUser500Response {
			w.WriteHeader(500)
			return
		}

		jsonified, err := xj.Convert(r.Body)
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
		w.Write(body)
	}))
	return vsphereMock
}

func routeBody(t *testing.T, requestType string, body map[string]interface{}) ([]byte, error) {
	switch requestType {
	case "RetrieveServiceContent":
		return loadResponse(t, "service-content.xml")
	case "Login":
		return loadResponse(t, "login.xml")
	case "Logout":
		return loadResponse(t, "logout.xml")
	case "RetrieveProperties":
		return routeRetreiveProperties(t, body)
	case "QueryPerf":
		return routePerformanceQuery(t, body)
	}

	return []byte{}, errNotFound
}

func routeRetreiveProperties(t *testing.T, body map[string]interface{}) ([]byte, error) {
	rp, ok := body["RetrieveProperties"].(map[string]interface{})
	require.True(t, ok)
	specSet := rp["specSet"].(map[string]interface{})

	var objectSetArray = false
	objectSet, ok := specSet["objectSet"].(map[string]interface{})
	if !ok {
		objectSetArray = true
	}

	var propSetArray = false
	propSet, ok := specSet["propSet"].(map[string]interface{})
	if !ok {
		propSetArray = true
	}

	var obj map[string]interface{}
	var content string
	var contentType string
	if !objectSetArray {
		obj = objectSet["obj"].(map[string]interface{})
		content = obj["#content"].(string)
		contentType = obj["-type"].(string)
	}

	switch {
	case content == "group-d1" && contentType == "Folder":
		return loadResponse(t, "datacenter.xml")

	case content == "datacenter-3" && contentType == "Datacenter":
		return loadResponse(t, "datacenter-properties.xml")

	case content == "domain-c8" && contentType == "ClusterComputeResource":
		if propSetArray {
			pSet := specSet["propSet"].([]interface{})
			for _, prop := range pSet {
				spec := prop.(map[string]interface{})
				specType := spec["type"].(string)
				if specType == "ResourcePool" {
					return loadResponse(t, "resource-pool.xml")
				}
			}
		}
		path := propSet["pathSet"].(string)
		switch path {
		case "datastore":
			return loadResponse(t, "cluster-datastore.xml")
		case "summary":
			return loadResponse(t, "cluster-summary.xml")
		case "host":
			return loadResponse(t, "host-list.xml")
		}

	case content == "PerfMgr" && contentType == "PerformanceManager":
		return loadResponse(t, "perf-manager.xml")

	case content == "group-h5" && contentType == "Folder":
		// TODO: look into propset for when to grab the parent resource
		if objectSet["skip"] == "true" {
			return loadResponse(t, "host-cluster.xml")
		}
		return loadResponse(t, "host-parent.xml")

	case content == "datastore-1003" && contentType == "Datastore":
		if objectSetArray {
			return loadResponse(t, "datastore-list.xml")
		}
		return loadResponse(t, "datastore-summary.xml")

	case contentType == "HostSystem":
		if ps, ok := propSet["pathSet"].([]interface{}); ok {
			for _, v := range ps {
				if v == "summary.hardware" {
					return loadResponse(t, "host-properties.xml")
				}
			}
		} else {
			ps, ok := propSet["pathSet"].(string)
			require.True(t, ok)
			if ps == "name" {
				return loadResponse(t, "host-names.xml")
			}

		}

	case content == "group-v4" && contentType == "Folder":
		if propSetArray {
			return loadResponse(t, "vm-group.xml")
		}
		if propSet == nil {
			return loadResponse(t, "vm-folder.xml")
		}
		return loadResponse(t, "vm-folder-parent.xml")

	case content == "vm-1040" && contentType == "VirtualMachine":
		if propSet["pathSet"] == "summary.runtime.host" {
			return loadResponse(t, "vm-host.xml")
		}
		return loadResponse(t, "vm-properties.xml")

	case (content == "group-v1034" || content == "group-v1001") && contentType == "Folder":
		return loadResponse(t, "vm-empty-folder.xml")

	case contentType == "ResourcePool":
		if ps, ok := propSet["pathSet"].([]interface{}); ok {
			for _, prop := range ps {
				if prop == "summary" {
					return loadResponse(t, "resource-pool-summary.xml")
				}
			}
		}

		if ss, ok := objectSet["selectSet"].(map[string]interface{}); ok && ss["path"] == "resourcePool" {
			return loadResponse(t, "resource-pool-group.xml")
		}

	case objectSetArray:
		objectArray := specSet["objectSet"].([]interface{})
		for _, i := range objectArray {
			m, ok := i.(map[string]interface{})
			require.True(t, ok)
			mObj := m["obj"].(map[string](interface{}))
			typeString := mObj["-type"]
			if typeString == "HostSystem" {
				return loadResponse(t, "host-names.xml")
			}
		}
	}

	return []byte{}, errNotFound
}

func routePerformanceQuery(t *testing.T, body map[string]interface{}) ([]byte, error) {
	queryPerf := body["QueryPerf"].(map[string]interface{})
	require.NotNil(t, queryPerf)
	querySpec := queryPerf["querySpec"].(map[string]interface{})
	entity := querySpec["entity"].(map[string]interface{})
	switch entity["-type"] {
	case "HostSystem":
		return loadResponse(t, "host-performance-counters.xml")
	case "VirtualMachine":
		return loadResponse(t, "vm-performance-counters.xml")
	}

	return []byte{}, errNotFound
}

func loadResponse(t *testing.T, filename string) ([]byte, error) {
	return responses.ReadFile(filepath.Join("internal", "mockserver", "responses", filename))
}
