// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otto

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestJSONToYAML_ServeHTTP(t *testing.T) {
	logBuf := bytes.NewBuffer(nil)
	h := jsonToYAMLHandler{
		logger: log.New(logBuf, "", 0),
	}
	expected := testPerson{
		Name: "Bob",
		Age:  42,
	}
	jsonBytes, err := json.Marshal(expected)
	require.NoError(t, err)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/jsonToYAML", bytes.NewBuffer(jsonBytes))
	h.ServeHTTP(resp, req)
	result := resp.Result()
	assert.Equal(t, http.StatusOK, result.StatusCode)
	yml, err := io.ReadAll(result.Body)
	require.NoError(t, err)
	actual := testPerson{}
	err = yaml.UnmarshalStrict(yml, &actual)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
	assert.Empty(t, logBuf)
}

func TestJSONToYAML_ServeHTTP_ReadError(t *testing.T) {
	logBuf := bytes.NewBuffer(nil)
	h := jsonToYAMLHandler{
		logger: log.New(logBuf, "", 0),
	}
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/jsonToYAML", failingReader{})
	h.ServeHTTP(resp, req)
	assert.Contains(t, logBuf.String(), "error reading request")
}

func TestJSONToYAML_ServeHTTP_UnmarshalingError(t *testing.T) {
	logBuf := bytes.NewBuffer(nil)
	h := jsonToYAMLHandler{
		logger: log.New(logBuf, "", 0),
	}
	const badJSON = `{`
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/jsonToYAML", bytes.NewBuffer([]byte(badJSON)))
	h.ServeHTTP(resp, req)
	const expected = "error unmarshaling request: unexpected end of JSON input"
	assert.Contains(t, logBuf.String(), expected)
	result := resp.Result()
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
}

type testPerson struct {
	Name string `mapstructure:"name" json:"name"`
	Age  int    `mapstructure:"age" json:"age"`
}

type failingReader struct{}

func (f failingReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("oops")
}
