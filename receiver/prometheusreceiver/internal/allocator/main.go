package allocator

import (
	promHTTP "github.com/prometheus/prometheus/discovery/http"
)

type Client struct {
	Endpoint     string
	CollectorID  string
	Interval     int32
	HTTPSDConfig promHTTP.SDConfig
}
