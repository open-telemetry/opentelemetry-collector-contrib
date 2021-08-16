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

// This file will be used in the end_to_end test for prometheus_exec receiver
// It acts as a Prometheus exporter, exposing an endpoint to be scraped with metrics
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	server()
}

// server serves one route "./metrics" and will shutdown the server as soon as it is scraped once, to allow for the next subprocess to be run
func server() {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(fmt.Sprintf("# HELP timestamp_now Unix timestamp\n# TYPE timestamp_now gauge\ntimestamp_now %v", strconv.FormatInt(time.Now().UnixNano(), 10))))
		if err != nil {
			log.Fatal(err)
		}

		// Schedule termination for this program in 100ms.
		go func() {
			time.Sleep(100 * time.Millisecond)
			os.Exit(1)
		}()
	})

	err := http.ListenAndServe(fmt.Sprintf(":%v", os.Args[1]), nil)
	log.Fatal(err)
}
