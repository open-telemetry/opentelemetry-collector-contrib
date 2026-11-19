// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strconv"
	"strings"
	"testing"
)

var benchTags = []string{
	"env:production",
	"service:api-server",
	"region:us-west-2",
	"availability_zone:us-west-2b",
	"kube_namespace:default",
	"kube_cluster_name:prod-cluster-1",
	"pod_name:api-server-7d9f8c6b5-x2k4p",
	"container_name:api-server",
	"version:2024.11.3",
	"team:backend",
	"http.method:POST",
	"http.route:/api/v1/items/{id}",
	"http.status_code:200",
	"rpc.system:grpc",
	"rpc.method:ItemService/Get",
	"db.system:postgresql",
	"db.operation:SELECT",
	"cloud.provider:aws",
	"host:ip-10-12-34-56.us-west-2.compute.internal",
	"deployment_id:d-8f3a2c91e7b4",
}

func BenchmarkParseMessageToMetric(b *testing.B) {
	for _, n := range []int{0, 4, 12, 20} {
		line := "http.server.request.count:1|c"
		if n > 0 {
			line += "|#" + strings.Join(benchTags[:n], ",")
		}
		b.Run("tags"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := parseMessageToMetric(line, true, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
