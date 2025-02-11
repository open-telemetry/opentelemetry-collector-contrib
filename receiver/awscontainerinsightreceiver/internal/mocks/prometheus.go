// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mocks // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/mocks"

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"

	gokitlog "github.com/go-kit/log"
	promcfg "github.com/prometheus/prometheus/config"
	"gopkg.in/yaml.v2"
)

type MockPrometheusResponse struct {
	Code           int
	Data           string
	UseOpenMetrics bool
}

type MockPrometheus struct {
	Mu          sync.Mutex // mu protects the fields below.
	Endpoints   map[string][]MockPrometheusResponse
	AccessIndex map[string]*atomic.Int32
	Wg          *sync.WaitGroup
	Srv         *httptest.Server
}
type TestData struct {
	Name  string
	Pages []MockPrometheusResponse
}

func (mp *MockPrometheus) Close() {
	mp.Srv.Close()
}

func (mp *MockPrometheus) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	mp.Mu.Lock()
	defer mp.Mu.Unlock()
	iptr, ok := mp.AccessIndex[req.URL.Path]
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		return
	}
	index := int(iptr.Load())
	iptr.Add(1)
	pages := mp.Endpoints[req.URL.Path]
	if index >= len(pages) {
		if index == len(pages) {
			mp.Wg.Done()
		}
		rw.WriteHeader(http.StatusNotFound)
		return
	}
	if pages[index].UseOpenMetrics {
		rw.Header().Set("Content-Type", "application/openmetrics-text")
	}
	rw.WriteHeader(pages[index].Code)
	_, _ = rw.Write([]byte(pages[index].Data))
}

func SetupMockPrometheus(tds ...*TestData) (*MockPrometheus, *promcfg.Config, error) {
	jobs := make([]map[string]any, 0, len(tds))
	endpoints := make(map[string][]MockPrometheusResponse)
	var metricPaths []string
	for _, t := range tds {
		metricPath := fmt.Sprintf("/%s/metrics", t.Name)
		endpoints[metricPath] = t.Pages
		metricPaths = append(metricPaths, metricPath)
	}
	mp := newMockPrometheus(endpoints)

	u, _ := url.Parse(mp.Srv.URL)
	for i := 0; i < len(tds); i++ {
		job := make(map[string]any)
		job["job_name"] = tds[i].Name
		job["metrics_path"] = metricPaths[i]
		job["scrape_interval"] = "1s"
		job["scrape_timeout"] = "500ms"
		job["static_configs"] = []map[string]any{{"targets": []string{u.Host}}}
		jobs = append(jobs, job)
	}
	if len(jobs) != len(tds) {
		log.Fatal("len(jobs) != len(targets), make sure job names are unique")
	}
	configP := make(map[string]any)
	configP["scrape_configs"] = jobs
	cfg, err := yaml.Marshal(&configP)
	if err != nil {
		return mp, nil, err
	}

	pCfg, err := promcfg.Load(string(cfg), false, gokitlog.NewNopLogger())
	return mp, pCfg, err
}

func newMockPrometheus(endpoints map[string][]MockPrometheusResponse) *MockPrometheus {
	accessIndex := make(map[string]*atomic.Int32)
	wg := &sync.WaitGroup{}
	wg.Add(len(endpoints))
	for k := range endpoints {
		accessIndex[k] = &atomic.Int32{}
	}
	mp := &MockPrometheus{
		Wg:          wg,
		AccessIndex: accessIndex,
		Endpoints:   endpoints,
	}
	srv := httptest.NewServer(mp)
	mp.Srv = srv
	return mp
}
