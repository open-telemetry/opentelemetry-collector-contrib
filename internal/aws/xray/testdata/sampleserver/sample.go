// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
)

func main() {
	// https://docs.aws.amazon.com/xray/latest/devguide/xray-sdk-go-handler.html
	mux := http.NewServeMux()
	mux.Handle("/", xray.Handler(
		xray.NewFixedSegmentNamer("SampleServer"), http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:8000", nil)
	if err != nil {
		panic(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
}
