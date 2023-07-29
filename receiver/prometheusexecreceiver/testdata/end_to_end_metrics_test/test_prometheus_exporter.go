// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
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

	server := &http.Server{
		Addr:              fmt.Sprintf(":%v", os.Args[1]),
		Handler:           mux,
		ReadHeaderTimeout: 20 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
