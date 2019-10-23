// Copyright 2019, OpenTelemetry Authors
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

package local

import (
	"sync"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	tagKeyMsgType, _ = tag.NewKey("msg_type")
	tagKeyPhase, _   = tag.NewKey("phase")

	defaultSizeDistribution    = view.Distribution(0, 1024, 2048, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, 67108864, 268435456, 1073741824, 4294967296)
	defaultLatencyDistribution = view.Distribution(0, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000)

	msgLatency   = stats.Float64("opencensus.io/php_daemon/latency", "Distribution of end to end latencies", stats.UnitMilliseconds)
	msgReqCount  = stats.Int64("opencensus.io/php_daemon/request_count", "Number of received messages", stats.UnitDimensionless)
	msgProcCount = stats.Int64("opencensus.io/php_daemon/process_count", "Number of processed messages", stats.UnitDimensionless)
	msgDropCount = stats.Int64("opencensus.io/php_daemon/drop_count", "Number of dropped messages", stats.UnitDimensionless)
	msgSize      = stats.Int64("opencensus.io/php_daemon/message_size", "Size of messages", stats.UnitBytes)

	viewLatency = &view.View{
		Name:        "opencensus.io/php_daemon/latency",
		Measure:     msgLatency,
		Description: "The distribution of end to end latencies",
		TagKeys:     []tag.Key{tagKeyMsgType, tagKeyPhase},
		Aggregation: defaultLatencyDistribution,
	}
	viewReqCount = &view.View{
		Name:        "opencensus.io/php_daemon/requests_received",
		Measure:     msgReqCount,
		Description: "The number of received requests",
		TagKeys:     []tag.Key{tagKeyMsgType},
		Aggregation: view.Count(),
	}
	viewProcCount = &view.View{
		Name:        "opencensus.io/php_daemon/requests_processed",
		Measure:     msgProcCount,
		Description: "The number of processed requests",
		TagKeys:     []tag.Key{tagKeyMsgType},
		Aggregation: view.Count(),
	}
	viewDropCount = &view.View{
		Name:        "opencensus.io/php_daemon/requests_dropped",
		Measure:     msgDropCount,
		Description: "The number of dropped requests",
		TagKeys:     []tag.Key{tagKeyMsgType},
		Aggregation: view.Count(),
	}
	viewMsgSize = &view.View{
		Name:        "opencensus.io/php_daemon/request_size",
		Measure:     msgSize,
		Description: "Size distribution of received messages",
		TagKeys:     []tag.Key{tagKeyMsgType},
		Aggregation: defaultSizeDistribution,
	}

	registerViews sync.Once
)
