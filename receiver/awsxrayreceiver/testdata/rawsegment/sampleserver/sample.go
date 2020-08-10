// +build xraysegmentdump

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
