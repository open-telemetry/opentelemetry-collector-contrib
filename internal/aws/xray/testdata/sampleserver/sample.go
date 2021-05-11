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
	"net/http"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
)

func main() {
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-go-handler.html
	http.Handle("/", xray.Handler(
		xray.NewFixedSegmentNamer("SampleServer"), http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hello!"))
			},
		),
	))

	go http.ListenAndServe(":8000", nil)
	time.Sleep(time.Second)

	resp, err := http.Get("http://localhost:8000")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}
