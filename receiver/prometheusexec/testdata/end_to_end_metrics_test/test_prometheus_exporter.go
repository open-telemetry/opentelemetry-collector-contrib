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

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// This file will be used in the end_to_end test for prometheus_exec receiver
// It acts as a Prometheus exporter, exposing an endpoint to be scraped with metrics
func main() {
	writeMetrics()
	server()
}

// writeMetrics truncates the previous metrics file or creates it if not there, populating it with the current time as a metric
func writeMetrics() {
	f, err := os.Create("./testdata/metrics")
	if err != nil {
		return
	}

	f.WriteString(fmt.Sprintf("# HELP timestamp_now Unix timestamp\n# TYPE timestamp_now gauge\ntimestamp_now %v", strconv.FormatInt(time.Now().Unix(), 10)))

	f.Close()
}

// server serves one route "./metrics" and will shutdown the server as soon as it is scraped once, to allow for the next subprocess to be run
func server() {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		defer os.Exit(1)
		http.ServeFile(w, r, "./testdata/metrics")
		return
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", os.Args[1]), nil))
}
