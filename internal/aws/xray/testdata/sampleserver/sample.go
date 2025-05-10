// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net/http"
	"time"

	"github.com/aws/aws-xray-sdk-go/v2/xray"
)

func main() {
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-go-handler.html
	mux := http.NewServeMux()
	mux.Handle("/", xray.Handler(
		xray.NewFixedSegmentNamer("SampleServer"), http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("Hello!"))
			},
		),
	))
	server := &http.Server{
		Addr:              ":8000",
		Handler:           mux,
		ReadHeaderTimeout: 20 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	time.Sleep(time.Second)

	resp, err := http.Get("http://localhost:8000")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}
