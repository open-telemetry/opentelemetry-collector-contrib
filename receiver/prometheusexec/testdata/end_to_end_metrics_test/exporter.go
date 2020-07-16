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

func main() {
	writeMetrics()
	go server()
	time.Sleep(10 * time.Second)
	os.Exit(1)
}

func server() {
	http.Handle("/", http.FileServer(http.Dir("./testdata/")))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", os.Args[1]), nil))
}

// writeMetrics truncates the previous metricsfile or creates it if not there, populating it with the current time
func writeMetrics() {
	f, err := os.Create("./testdata/metrics")
	if err != nil {
		return
	}

	f.WriteString(fmt.Sprintf("# HELP timestamp_now Unix timestamp\n# TYPE timestamp_now gauge\ntimestamp_now %v", strconv.FormatInt(time.Now().Unix(), 10)))

	f.Close()
}
