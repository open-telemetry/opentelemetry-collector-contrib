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

package newrelicexporter

import (
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
)

type Data struct {
	Common          Common   `json:"common"`
	Spans           []Span   `json:"spans"`
	Metrics         []Metric `json:"metrics"`
	XXXUnrecognized []byte   `json:"-"`
}

type Common struct {
	Attributes      map[string]string `json:"attributes"`
	XXXUnrecognized []byte            `json:"-"`
}

type Span struct {
	ID              string                 `json:"id"`
	TraceID         string                 `json:"trace.id"`
	Attributes      map[string]interface{} `json:"attributes"`
	XXXUnrecognized []byte                 `json:"-"`
}

type Metric struct {
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	Value           interface{}            `json:"value"`
	Timestamp       int64                  `json:"timestamp"`
	Interval        int64                  `json:"interval"`
	Attributes      map[string]interface{} `json:"attributes"`
	XXXUnrecognized []byte                 `json:"-"`
}

// Mock caches decompressed request bodies
type Mock struct {
	Data []Data
}

func (c *Mock) Spans() []Span {
	var spans []Span
	for _, data := range c.Data {
		spans = append(spans, data.Spans...)
	}
	return spans
}

func (c *Mock) Metrics() []Metric {
	var metrics []Metric
	for _, data := range c.Data {
		metrics = append(metrics, data.Metrics...)
	}
	return metrics
}

func (c *Mock) Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// telemetry sdk gzip compresses json payloads
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer gz.Close()

		contents, err := ioutil.ReadAll(gz)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !json.Valid(contents) {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		err = c.ParseRequest(contents)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(200)
	}))
}

func (c *Mock) ParseRequest(b []byte) error {
	var data []Data
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	c.Data = append(c.Data, data...)
	return nil
}
